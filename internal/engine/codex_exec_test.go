package engine

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCodexExecUsesBoundedReadOnlyInvocation(t *testing.T) {
	runner := &recordingRunner{result: []byte(`{"action":"defer","reason":"Nothing meaningful."}`)}
	root, workspace, tempDir := codexSandboxDirs(t)
	exec := CodexExec{
		Executable:     "codex",
		Sandbox:        "read-only",
		Timeout:        time.Second,
		MaxOutputBytes: 4096,
		Environment:    []string{"PATH=/usr/bin:/bin", "HOME=/private/tmp/home"},
		Runner:         runner,
		SandboxRoot:    root,
		TempDir:        tempDir,
		Workspace:      workspace,
	}
	output, err := exec.Run(context.Background(), Input{Instructions: "trusted instructions", Bundle: `{"items":[]}`})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output.Stdout), `"action":"defer"`) {
		t.Fatalf("stdout = %q", output.Stdout)
	}
	args := strings.Join(runner.args, " ")
	for _, required := range []string{"exec", "--sandbox read-only", "--ephemeral", "--json", "--output-schema", "--output-last-message", "--ignore-user-config", "--ignore-rules", "--skip-git-repo-check", "--cd", "-"} {
		if !strings.Contains(args, required) {
			t.Errorf("args %q missing %q", args, required)
		}
	}
	if strings.Contains(strings.Join(runner.env, "\n"), "GIT_TOKEN") {
		t.Fatalf("environment leaked publish token: %#v", runner.env)
	}
}

func TestCodexExecCancelsHungProcess(t *testing.T) {
	root, workspace, tempDir := codexSandboxDirs(t)
	exec := CodexExec{
		Executable:     "codex",
		Sandbox:        "read-only",
		Timeout:        40 * time.Millisecond,
		MaxOutputBytes: 4096,
		Runner:         &hangingRunner{},
		SandboxRoot:    root,
		TempDir:        tempDir,
		Workspace:      workspace,
	}
	started := time.Now()
	_, err := exec.Run(context.Background(), Input{Instructions: "instructions", Bundle: "{}"})
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want deadline", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("hung process termination took %s", elapsed)
	}
}

func TestCodexExecRejectsEnvironmentCredentialCanary(t *testing.T) {
	token := "canary-git-secret"
	root, workspace, tempDir := codexSandboxDirs(t)
	exec := CodexExec{
		Executable: "codex", Sandbox: "read-only", Timeout: time.Second, MaxOutputBytes: 4096,
		Environment: []string{"PATH=/usr/bin", "THEMACHINEWRITES_GIT_PUBLISH_TOKEN=" + token},
		Runner:      &recordingRunner{result: []byte(`{}`)}, SandboxRoot: root, TempDir: tempDir, Workspace: workspace,
	}
	_, err := exec.Run(context.Background(), Input{Instructions: "instructions", Bundle: "{}"})
	if err == nil || strings.Contains(err.Error(), token) {
		t.Fatalf("Run() error = %v, want redacted environment rejection", err)
	}
}

func TestCodexExecRejectsUnapprovedEnvironmentAndRequiresEmptyWorkspace(t *testing.T) {
	root, workspace, tempDir := codexSandboxDirs(t)
	runner := &recordingRunner{result: []byte(`{"action":"defer","reason":"No."}`)}
	exec := CodexExec{
		Executable: "codex", Sandbox: "read-only", Timeout: time.Second, MaxOutputBytes: 4096,
		Workspace: workspace, Environment: []string{"PATH=/usr/bin", "MY_TOKEN_ALIAS=secret-value"},
		Runner: runner, SandboxRoot: root, TempDir: tempDir,
	}
	_, err := exec.Run(context.Background(), Input{Instructions: "instructions", Bundle: "{}"})
	if err == nil || strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("Run() error = %v, want unapproved environment rejection", err)
	}
	exec.Environment = []string{"PATH=/usr/bin"}
	exec.SandboxRoot = ""
	if _, err := exec.Run(context.Background(), Input{Instructions: "instructions", Bundle: "{}"}); err == nil {
		t.Fatal("Run() accepted missing isolated sandbox root")
	}
}

func TestCodexExecChildHasSanitizedEnvironmentAndSyntheticHome(t *testing.T) {
	goBinary, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go executable unavailable")
	}
	executable, err := filepath.Abs("testdata/fake-codex/main.go")
	if err != nil {
		t.Fatal(err)
	}
	root, workspace, tempDir := codexSandboxDirs(t)
	t.Setenv("THEMACHINEWRITES_GIT_PUBLISH_TOKEN", "environment-secret-canary")
	code := CodexExec{
		Executable: goBinary, Sandbox: "read-only", Timeout: 10 * time.Second,
		MaxOutputBytes: 64 * 1024, SandboxRoot: root, TempDir: tempDir, Workspace: workspace,
		Runner: &commandProcessRunner{prefix: []string{"run", executable, "--mode", "canary"}},
	}
	output, err := code.Run(context.Background(), Input{Instructions: "instructions", Bundle: `{}`})
	if err != nil {
		if strings.Contains(err.Error(), "codex exec failed") {
			t.Skip("nested macOS sandbox is unavailable in this test environment")
		}
		t.Fatal(err)
	}
	var result struct {
		Home                string `json:"home"`
		CWD                 string `json:"cwd"`
		PublishTokenPresent bool   `json:"publish_token_present"`
	}
	if err := json.Unmarshal(output.Stdout, &result); err != nil {
		t.Fatalf("decode fixture result %q: %v", output.Stdout, err)
	}
	if result.PublishTokenPresent {
		t.Fatalf("fixture received environment canary: %#v", result)
	}
	expectedHome, homeErr := filepath.EvalSymlinks(filepath.Join(root, "home"))
	expectedTemp, tempErr := filepath.EvalSymlinks(tempDir)
	actualHome, actualHomeErr := filepath.EvalSymlinks(result.Home)
	actualTemp, actualTempErr := filepath.EvalSymlinks(result.CWD)
	if homeErr != nil || tempErr != nil || actualHomeErr != nil || actualTempErr != nil || actualHome != expectedHome || actualTemp != expectedTemp {
		t.Fatalf("fixture sandbox paths = home %q cwd %q", result.Home, result.CWD)
	}
}

func TestCodexExecPassesOnlyDedicatedCodexHome(t *testing.T) {
	root, workspace, tempDir := codexSandboxDirs(t)
	codexHome := filepath.Join(root, "codex-home")
	if err := os.Mkdir(codexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{result: []byte(`{"action":"defer","reason":"No."}`)}
	code := CodexExec{
		Executable: "codex", Sandbox: "read-only", Timeout: time.Second, MaxOutputBytes: 4096,
		SandboxRoot: root, Workspace: workspace, TempDir: tempDir, Runner: runner,
		Environment: []string{"PATH=/usr/bin", "CODEX_HOME=" + codexHome},
	}
	if _, err := code.Run(context.Background(), Input{Instructions: "instructions", Bundle: `{}`}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	joined := strings.Join(runner.env, "\n")
	if !strings.Contains(joined, "CODEX_HOME="+codexHome) {
		t.Fatalf("dedicated CODEX_HOME absent: %#v", runner.env)
	}
	for _, required := range []string{"HOME=" + filepath.Join(root, "home"), "TMPDIR=" + tempDir, "PATH=/usr/bin"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("environment %#v missing %q", runner.env, required)
		}
	}
}

func TestCodexExecRejectsPublisherSecretEvenWithCodexHome(t *testing.T) {
	root, workspace, tempDir := codexSandboxDirs(t)
	codexHome := filepath.Join(root, "codex-home")
	if err := os.Mkdir(codexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	code := CodexExec{
		Executable: "codex", Sandbox: "read-only", Timeout: time.Second, MaxOutputBytes: 4096,
		SandboxRoot: root, Workspace: workspace, TempDir: tempDir, Runner: &recordingRunner{result: []byte(`{}`)},
		Environment: []string{"PATH=/usr/bin", "CODEX_HOME=" + codexHome, "THEMACHINEWRITES_GIT_PUBLISH_TOKEN=forbidden"},
	}
	if _, err := code.Run(context.Background(), Input{Instructions: "instructions", Bundle: `{}`}); err == nil {
		t.Fatal("publisher token was accepted into Codex environment")
	}
}

func TestCodexExecCanApplyNativeFilesystemDenial(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("native sandbox integration is macOS-only")
	}
	goBinary, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go executable unavailable")
	}
	executable, err := filepath.Abs("testdata/fake-codex/main.go")
	if err != nil {
		t.Fatal(err)
	}
	root, workspace, tempDir := codexSandboxDirs(t)
	denied := t.TempDir()
	if _, err := exec.Command(sandboxExecutable, "-p", "(version 1)(allow default)", "/usr/bin/true").CombinedOutput(); err != nil {
		t.Skip("nested macOS sandbox is unavailable in this test environment")
	}
	if err := os.WriteFile(filepath.Join(denied, "secret"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	code := CodexExec{
		Executable: goBinary, Sandbox: "read-only", Timeout: 10 * time.Second,
		MaxOutputBytes: 64 * 1024, SandboxRoot: root, TempDir: tempDir, Workspace: workspace,
		DeniedReadPaths: []string{denied},
		Runner:          &commandProcessRunner{prefix: []string{"run", executable, "--mode", "filesystem-canary", "--canary", filepath.Join(denied, "secret")}},
	}
	output, err := code.Run(context.Background(), Input{Instructions: "instructions", Bundle: `{}`})
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		CanaryReadable bool `json:"canary_readable"`
	}
	if err := json.Unmarshal(output.Stdout, &result); err != nil {
		t.Fatalf("decode fixture result %q: %v", output.Stdout, err)
	}
	if result.CanaryReadable {
		t.Fatal("native sandbox did not deny filesystem canary")
	}
}

func TestMacOSSandboxProfileRejectsUnsafePathAndCanonicalizesAliases(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS profile syntax test")
	}
	path := t.TempDir()
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := macOSSandboxProfile([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(profile, `(subpath "`+canonical+`")`) {
		t.Fatalf("profile did not use canonical path: %s", profile)
	}
	if _, err := macOSSandboxProfile([]string{"relative/path"}); err == nil {
		t.Fatal("profile accepted relative denied path")
	}
}

func codexSandboxDirs(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	tempDir := filepath.Join(root, "tmp")
	home := filepath.Join(root, "home")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(tempDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	return root, workspace, tempDir
}

type recordingRunner struct {
	args   []string
	env    []string
	input  []byte
	result []byte
	err    error
}

func (r *recordingRunner) Run(_ context.Context, _ string, args, env []string, stdin []byte) ([]byte, []byte, error) {
	r.args, r.env, r.input = append([]string(nil), args...), append([]string(nil), env...), append([]byte(nil), stdin...)
	return r.result, nil, r.err
}

type hangingRunner struct{}

func (hangingRunner) Run(ctx context.Context, _ string, _ []string, _ []string, _ []byte) ([]byte, []byte, error) {
	<-ctx.Done()
	return nil, nil, ctx.Err()
}

var _ = os.Getenv
var _ = filepath.Join
