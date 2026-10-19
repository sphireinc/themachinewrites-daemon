package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/config"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/sources"
)

func TestFactoryBuildsDryRunCycleWithoutPublisher(t *testing.T) {
	directory := t.TempDir()
	codexHome := filepath.Join(directory, "codex-home")
	runtimeDir := filepath.Join(directory, "runtime")
	public := filepath.Join(directory, "public")
	for _, path := range []string{codexHome, runtimeDir, public} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Config{
		PollInterval: time.Minute, SourcesPath: filepath.Join(directory, "sources.yaml"), RuntimeDir: runtimeDir,
		CodexExecutable: "codex", CodexTimeout: time.Second, CodexModel: "gpt-6", CodexHome: codexHome,
		PublicRepoPath: public, PublicBranch: "main", PublicRemote: "https://github.com/example/public.git",
		Budget: config.Budget{DailyModelInvocations: 1, DailyInputTokens: 1000, DailyOutputTokens: 1000, MaxAttempts: 1},
		DryRun: true,
	}
	cycle, closeRuntime, err := buildSupervisorCycleWith(context.Background(), cfg, sources.Configuration{}, nil, func() time.Time { return time.Unix(0, 0).UTC() })
	if err != nil {
		t.Fatal(err)
	}
	defer closeRuntime()
	if !cycle.DryRun || cycle.Poll == nil || cycle.Decide == nil {
		t.Fatalf("dry-run cycle = %#v", cycle)
	}
	if _, err := os.Stat(filepath.Join(runtimeDir, "runtime.sqlite")); err != nil {
		t.Fatalf("runtime database missing: %v", err)
	}
}
