package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/config"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/sources"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/supervisor"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/telemetry"
)

func TestRunDispatchesSupportedCommands(t *testing.T) {
	directory := t.TempDir()
	sourcesPath := filepath.Join(directory, "sources.yaml")
	if err := os.WriteFile(sourcesPath, []byte("sources:\n  - {id: fixture, url: https://example.org/feed, enabled: true, request_interval: 15m, name: Fixture}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(directory, "daemon.yaml")
	codexHome := filepath.Join(directory, "codex-home")
	if err := os.Mkdir(codexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	public := filepath.Join(directory, "public")
	if err := os.Mkdir(public, 0o700); err != nil {
		t.Fatal(err)
	}
	runtimeDirectory := filepath.Join(directory, "runtime")
	if err := os.Mkdir(runtimeDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	content := "dry_run: true\nsources_path: " + sourcesPath + "\nruntime_dir: " + runtimeDirectory + "\ncodex_model: gpt-6\ncodex_home: " + codexHome + "\npublic_repo_path: " + public + "\npublic_branch: main\npublic_remote: https://github.com/example/public.git\n"
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	called := false
	factory := func(_ context.Context, cfg config.Config, _ sources.Configuration) (supervisor.Cycle, func() error, error) {
		called = true
		return supervisor.Cycle{DryRun: cfg.DryRun, Poll: func(context.Context) []error { return nil }}, func() error { return nil }, nil
	}
	var stdout, stderr bytes.Buffer
	if code := runWithFactory([]string{"once", "--config", configPath}, &stdout, &stderr, factory); code != 0 {
		t.Fatalf("once exit=%d stderr=%s", code, stderr.String())
	}
	if !called {
		t.Fatal("once did not construct the supervisor cycle")
	}
	if !strings.Contains(stdout.String(), "once complete") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestRunHelpListsEverySubcommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help exit code = %d, stderr = %q", code, stderr.String())
	}
	for _, command := range []string{"run", "once", "validate-config", "status", "backup"} {
		if !strings.Contains(stdout.String(), command) {
			t.Errorf("help output %q does not mention %q", stdout.String(), command)
		}
	}
}

func TestStatusPrintsPausedReasonAndLastTimes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "health.json")
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	if err := telemetry.WriteHealth(path, telemetry.Health{LastConfirmedPush: &at, PausedReason: "budget_limit"}); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"status", "--snapshot", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("status exit=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{"paused_reason: budget_limit", "last_confirmed_push: 2026-10-18T20:00:00Z", "last_poll: never"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("status output missing %q: %s", want, stdout.String())
		}
	}
}

func TestBackupCommandCreatesVerifiedRestorableFile(t *testing.T) {
	directory := t.TempDir()
	codexHome := filepath.Join(directory, "codex-home")
	runtimeDir := filepath.Join(directory, "runtime")
	public := filepath.Join(directory, "public")
	for _, path := range []string{codexHome, runtimeDir, public} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(directory, "daemon.yaml")
	content := "dry_run: true\nsources_path: sources.yaml\nruntime_dir: " + runtimeDir + "\ncodex_model: gpt-6\ncodex_home: " + codexHome + "\npublic_repo_path: " + public + "\npublic_branch: main\npublic_remote: https://github.com/example/public.git\n"
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime, err := store.Open(context.Background(), filepath.Join(runtimeDir, "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.DB().ExecContext(context.Background(), "INSERT INTO memories(id,kind,summary,updated_at,source_hash) VALUES ('m1','observation','summary','2026-10-18T20:00:00Z','hash')"); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Close(); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(directory, "backup.sqlite")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"backup", "--config", configPath, "--output", output}, &stdout, &stderr); code != 0 {
		t.Fatalf("backup exit=%d stderr=%s", code, stderr.String())
	}
	restored, err := store.Open(context.Background(), output)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var count int
	if err := restored.DB().QueryRow("SELECT count(*) FROM memories").Scan(&count); err != nil || count != 1 {
		t.Fatalf("restored count = %d, %v", count, err)
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"unknown"}, &stdout, &stderr); code == 0 {
		t.Fatal("unknown command exited zero")
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q, want unknown-command message", stderr.String())
	}
}

func TestValidateConfigLoadsDryRunWithoutCredentials(t *testing.T) {
	directory := t.TempDir()
	codexHome := filepath.Join(directory, "codex-home")
	if err := os.Mkdir(codexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "daemon.yaml")
	content := "dry_run: true\nsources_path: sources.yaml\nruntime_dir: runtime\n" +
		"codex_model: gpt-6\ncodex_home: " + codexHome + "\npublic_repo_path: public\npublic_branch: main\npublic_remote: https://github.com/example/public.git\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"validate-config", "--config", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("validate-config exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "configuration valid") || !strings.Contains(stdout.String(), "dry_run") {
		t.Fatalf("stdout = %q, want redacted valid configuration summary", stdout.String())
	}
}

func TestValidateConfigFailureDoesNotLeakYAMLCredential(t *testing.T) {
	const canary = "yaml-credential-canary"
	path := filepath.Join(t.TempDir(), "daemon.yaml")
	content := "git_publish_token: " + canary + "\ndry_run: true\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"validate-config", "--config", path}, &stdout, &stderr); code == 0 {
		t.Fatal("validate-config accepted a YAML credential field")
	}
	combined := stdout.String() + stderr.String()
	if strings.Contains(combined, canary) {
		t.Fatalf("validate-config leaked YAML credential: %q", combined)
	}
}

func TestValidateConfigFailsForLiveModeWithoutPublishCredential(t *testing.T) {
	directory := t.TempDir()
	repository := filepath.Join(directory, "public")
	codexHome := filepath.Join(directory, "codex-home")
	if err := os.Mkdir(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(codexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "daemon.yaml")
	content := "dry_run: false\nsources_path: sources.yaml\nruntime_dir: runtime\n" +
		"codex_model: gpt-6\ncodex_home: " + codexHome + "\npublic_repo_path: " + repository + "\npublic_branch: main\npublic_remote: https://github.com/example/public.git\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"validate-config", "--config", path}, &stdout, &stderr); code == 0 {
		t.Fatal("live configuration accepted without publication credential")
	}
	if !strings.Contains(stderr.String(), "THEMACHINEWRITES_GIT_PUBLISH_TOKEN") {
		t.Fatalf("stderr = %q, want missing-credential variable name", stderr.String())
	}
}
