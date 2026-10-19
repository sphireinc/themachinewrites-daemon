package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/config"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/logging"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/sources"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/supervisor"
)

const usage = `Usage: themachinewrites <command>

Commands:
  run              run the supervisor until canceled
  once             execute one bounded cycle
  validate-config  validate daemon configuration
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}

	switch args[0] {
	case "validate-config":
		return runValidateConfig(args[1:], stdout, stderr)
	case "run", "once":
		return runSupervisor(args[0], args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func runSupervisor(command string, args []string, stdout, stderr io.Writer) int {
	path, ok := configPath(args)
	if !ok {
		fmt.Fprintln(stderr, "usage: themachinewrites "+command+" [--config PATH]")
		return 2
	}
	cfg, err := config.LoadEnvy(path, os.Getenv)
	if err != nil {
		logging.New(stderr).Error("supervisor configuration failed", "event", "supervisor.config", "error", err)
		return 1
	}
	sourceConfig, err := sources.LoadConfiguration(cfg.SourcesPath)
	if err != nil {
		logging.New(stderr).Error("source configuration failed", "event", "supervisor.sources", "error", err)
		return 1
	}
	cycle := supervisor.Cycle{DryRun: cfg.DryRun, Poll: func(context.Context) []error { return nil }}
	adapter := loopAdapter{cycle: cycle, interval: cfg.PollInterval, backoff: supervisor.NewBackoff(time.Second, cfg.PollInterval, nil)}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if command == "once" {
		if err := adapter.Once(ctx); err != nil {
			logging.New(stderr).Error("supervisor cycle failed", "event", "supervisor.once", "error", err)
			return 1
		}
		fmt.Fprintf(stdout, "once complete (dry_run=%t, enabled_sources=%d)\n", cfg.DryRun, enabledCount(sourceConfig))
		return 0
	}
	fmt.Fprintf(stdout, "supervisor starting (dry_run=%t, enabled_sources=%d)\n", cfg.DryRun, enabledCount(sourceConfig))
	if err := adapter.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logging.New(stderr).Error("supervisor stopped", "event", "supervisor.run", "error", err)
		return 1
	}
	return 0
}

func enabledCount(configuration sources.Configuration) int {
	count := 0
	for _, source := range configuration.Sources {
		if source.Enabled {
			count++
		}
	}
	return count
}

func runValidateConfig(args []string, stdout, stderr io.Writer) int {
	logger := logging.New(stderr)
	path, ok := configPath(args)
	if !ok {
		logger.Error("invalid validate-config arguments", "event", "config.validate")
		fmt.Fprintln(stderr, "usage: themachinewrites validate-config [--config PATH]")
		return 2
	}
	cfg, err := config.LoadEnvy(path, os.Getenv)
	if err != nil {
		logger.Error("configuration validation failed", "event", "config.validate", "error", err)
		return 1
	}
	secrets, err := config.LoadSecrets(os.Getenv)
	if err != nil {
		logger.Error("secret loading failed", "event", "config.secrets", "error", err)
		return 1
	}
	if err := secrets.ValidateForPublication(!cfg.DryRun); err != nil {
		logger.Error("secret validation failed", "event", "config.secrets", "error", err)
		return 1
	}
	fmt.Fprintf(stdout, "configuration valid (dry_run=%t, publish_token_configured=%t)\n", cfg.DryRun, secrets.GitPublishToken != "")
	return 0
}

func configPath(args []string) (string, bool) {
	if len(args) == 0 {
		return "config.yaml", true
	}
	if len(args) == 2 && args[0] == "--config" && args[1] != "" {
		return args[1], true
	}
	return "", false
}
