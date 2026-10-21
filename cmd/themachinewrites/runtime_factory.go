package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/config"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/daemon"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/gitpub"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/logging"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/sources"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/supervisor"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/telemetry"
)

func buildSupervisorCycle(ctx context.Context, cfg config.Config, sourceConfig sources.Configuration) (supervisor.Cycle, func() error, error) {
	return buildSupervisorCycleWith(ctx, cfg, sourceConfig, os.Getenv, time.Now)
}

func buildSupervisorCycleWith(
	ctx context.Context,
	cfg config.Config,
	sourceConfig sources.Configuration,
	getenv func(string) string,
	now func() time.Time,
) (supervisor.Cycle, func() error, error) {
	if now == nil {
		return supervisor.Cycle{}, func() error { return nil }, errors.New("clock is required")
	}
	runtime, err := store.Open(ctx, filepath.Join(cfg.RuntimeDir, "runtime.sqlite"))
	if err != nil {
		return supervisor.Cycle{}, func() error { return nil }, err
	}
	closeRuntime := func() error { return runtime.Close() }
	if _, err := runtime.RequeueInterruptedCandidates(ctx, now()); err != nil {
		closeRuntime()
		return supervisor.Cycle{}, func() error { return nil }, err
	}
	fetcher := sources.NewFetcher(sources.NewURLValidator(nil, false))
	codexSandbox, err := os.MkdirTemp(cfg.RuntimeDir, "codex-sandbox-")
	if err != nil {
		closeRuntime()
		return supervisor.Cycle{}, func() error { return nil }, err
	}
	codexEngine, err := engine.NewBoundedCodexEngine(engine.FactoryConfig{
		Executable: cfg.CodexExecutable, Model: cfg.CodexModel, Timeout: cfg.CodexTimeout,
		SandboxRoot: codexSandbox, CodexHome: cfg.CodexHome,
		DeniedReadPaths: codexDeniedReadPaths(cfg),
	})
	if err != nil {
		os.RemoveAll(codexSandbox)
		closeRuntime()
		return supervisor.Cycle{}, func() error { return nil }, err
	}
	metrics := &telemetry.Metrics{}
	cycleRuntime := daemon.Runtime{
		Sources: sourceConfig, Fetcher: fetcher, Store: runtime, Engine: codexEngine,
		PublicRoot: cfg.PublicRepoPath, HealthPath: filepath.Join(cfg.RuntimeDir, "health.json"),
		ModelName: cfg.CodexModel, Now: now, DryRun: cfg.DryRun,
		Logger:  logging.New(os.Stderr),
		Metrics: metrics, MaxCandidates: 20, MaxOutputTokens: cfg.Budget.DailyOutputTokens,
		Budget: supervisor.NewBudget(now(), supervisor.BudgetLimits{
			Invocations: cfg.Budget.DailyModelInvocations, InputTokens: cfg.Budget.DailyInputTokens,
			OutputTokens: cfg.Budget.DailyOutputTokens, MaxRetries: cfg.Budget.MaxAttempts,
		}),
	}
	if !cfg.DryRun {
		token := ""
		if getenv != nil {
			token = getenv("THEMACHINEWRITES_GIT_PUBLISH_TOKEN")
		}
		if strings.TrimSpace(token) == "" {
			closeRuntime()
			return supervisor.Cycle{}, func() error { return nil }, errors.New("THEMACHINEWRITES_GIT_PUBLISH_TOKEN is required")
		}
		repository, err := gitpub.AuthenticatedRepository(ctx, gitpub.RepositoryConfig{
			Root: cfg.PublicRepoPath, Branch: cfg.PublicBranch, Remote: "origin", ExpectedRemote: cfg.PublicRemote,
		}, token)
		if err != nil {
			closeRuntime()
			return supervisor.Cycle{}, func() error { return nil }, err
		}
		cycleRuntime.Publisher = repository
		cycleRuntime.Outbox = runtime
	}
	cycle := supervisor.Cycle{DryRun: cfg.DryRun, Poll: cycleRuntime.Poll, Decide: cycleRuntime.DecideAndPublish}
	return cycle, func() error {
		removeErr := os.RemoveAll(codexSandbox)
		closeErr := closeRuntime()
		return errors.Join(removeErr, closeErr)
	}, nil
}

func codexDeniedReadPaths(cfg config.Config) []string {
	candidates := []string{
		filepath.Dir(cfg.PublicRepoPath),
		filepath.Dir(cfg.SourcesPath),
	}
	var result []string
	seen := map[string]bool{}
	for _, path := range candidates {
		absolute, err := filepath.Abs(path)
		if err != nil || !filepath.IsAbs(absolute) || seen[absolute] {
			continue
		}
		if info, err := os.Stat(absolute); err != nil || !info.IsDir() {
			continue
		}
		seen[absolute] = true
		result = append(result, absolute)
	}
	return result
}
