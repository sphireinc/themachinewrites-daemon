package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCodexExecUsesBoundedReadOnlyInvocation(t *testing.T) {
	runner := &recordingRunner{result: []byte(`{"action":"defer","reason":"Nothing meaningful."}`)}
	exec := CodexExec{
		Executable:     "codex",
		Sandbox:        "read-only",
		Timeout:        time.Second,
		MaxOutputBytes: 4096,
		Environment:    []string{"PATH=/usr/bin:/bin", "HOME=/private/tmp/home"},
		Runner:         runner,
		TempDir:        t.TempDir(),
		Workspace:      t.TempDir(),
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
	exec := CodexExec{
		Executable:     "codex",
		Sandbox:        "read-only",
		Timeout:        40 * time.Millisecond,
		MaxOutputBytes: 4096,
		Runner:         &hangingRunner{},
		TempDir:        t.TempDir(),
		Workspace:      t.TempDir(),
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
	exec := CodexExec{
		Executable: "codex", Sandbox: "read-only", Timeout: time.Second, MaxOutputBytes: 4096,
		Environment: []string{"PATH=/usr/bin", "THEMACHINEWRITES_GIT_PUBLISH_TOKEN=" + token},
		Runner:      &recordingRunner{result: []byte(`{}`)}, TempDir: t.TempDir(), Workspace: t.TempDir(),
	}
	_, err := exec.Run(context.Background(), Input{Instructions: "instructions", Bundle: "{}"})
	if err == nil || strings.Contains(err.Error(), token) {
		t.Fatalf("Run() error = %v, want redacted environment rejection", err)
	}
}

func TestCodexExecRejectsUnapprovedEnvironmentAndRequiresEmptyWorkspace(t *testing.T) {
	workspace := t.TempDir()
	runner := &recordingRunner{result: []byte(`{"action":"defer","reason":"No."}`)}
	exec := CodexExec{
		Executable: "codex", Sandbox: "read-only", Timeout: time.Second, MaxOutputBytes: 4096,
		Workspace: workspace, Environment: []string{"PATH=/usr/bin", "MY_TOKEN_ALIAS=secret-value"},
		Runner: runner, TempDir: t.TempDir(),
	}
	_, err := exec.Run(context.Background(), Input{Instructions: "instructions", Bundle: "{}"})
	if err == nil || strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("Run() error = %v, want unapproved environment rejection", err)
	}
	exec.Environment = []string{"PATH=/usr/bin"}
	exec.Workspace = ""
	if _, err := exec.Run(context.Background(), Input{Instructions: "instructions", Bundle: "{}"}); err == nil {
		t.Fatal("Run() accepted missing isolated workspace")
	}
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
