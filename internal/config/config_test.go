package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestLoadReadsTypedConfiguration(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "daemon.yaml")
	content := `poll_interval: 15m
sources_path: ./sources.yaml
runtime_dir: ./runtime
codex_executable: codex
codex_timeout: 8m
public_repo_path: ./themachinewrites-public
budget:
  daily_model_invocations: 12
  daily_input_tokens: 120000
  daily_output_tokens: 24000
  max_attempts: 3
dry_run: true
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Config{
		PollInterval:    15 * time.Minute,
		SourcesPath:     "./sources.yaml",
		RuntimeDir:      "./runtime",
		CodexExecutable: "codex",
		CodexTimeout:    8 * time.Minute,
		PublicRepoPath:  "./themachinewrites-public",
		Budget: Budget{
			DailyModelInvocations: 12,
			DailyInputTokens:      120000,
			DailyOutputTokens:     24000,
			MaxAttempts:           3,
		},
		DryRun: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.yaml")
	if err := os.WriteFile(path, []byte("dry_run: true\nunknown_setting: enabled\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load() accepted an unknown key")
	}
}
