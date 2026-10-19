package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDispatchesSupportedCommands(t *testing.T) {
	directory := t.TempDir()
	sourcesPath := filepath.Join(directory, "sources.yaml")
	if err := os.WriteFile(sourcesPath, []byte("sources:\n  - {id: fixture, url: https://example.org/feed, enabled: true, request_interval: 15m, name: Fixture}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(directory, "daemon.yaml")
	content := "dry_run: true\nsources_path: " + sourcesPath + "\nruntime_dir: runtime\npublic_repo_path: public\n"
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"once", "--config", configPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("once exit=%d stderr=%s", code, stderr.String())
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
	for _, command := range []string{"run", "once", "validate-config"} {
		if !strings.Contains(stdout.String(), command) {
			t.Errorf("help output %q does not mention %q", stdout.String(), command)
		}
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
	path := filepath.Join(directory, "daemon.yaml")
	content := "dry_run: true\nsources_path: sources.yaml\nruntime_dir: runtime\n" +
		"public_repo_path: public\n"
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
	if err := os.Mkdir(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "daemon.yaml")
	content := "dry_run: false\nsources_path: sources.yaml\nruntime_dir: runtime\n" +
		"public_repo_path: " + repository + "\n"
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
