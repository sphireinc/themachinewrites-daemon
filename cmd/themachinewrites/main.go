package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/config"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/gitpub"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/logging"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/operations"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/sources"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/supervisor"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/telemetry"
)

type supervisorFactory func(context.Context, config.Config, sources.Configuration) (supervisor.Cycle, func() error, error)

type repositoryOpener func(context.Context, config.Config) (*gitpub.Repository, error)

func openConfiguredRepository(ctx context.Context, cfg config.Config) (*gitpub.Repository, error) {
	return gitpub.OpenRepository(ctx, gitpub.RepositoryConfig{
		Root: cfg.PublicRepoPath, Branch: cfg.PublicBranch, Remote: "origin", ExpectedRemote: cfg.PublicRemote,
	}, nil)
}

const usage = `Usage: themachinewrites <command>

Commands:
  run              run the supervisor until canceled
  once             execute one bounded cycle
  validate-config  validate daemon configuration
  status           print local daemon health snapshot
  backup           create a verified online SQLite backup
  reconcile-public rebuild publication acknowledgements from remote Git history
  divergence       report local versus remote publication discrepancies
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	return runWithFactory(args, stdout, stderr, buildSupervisorCycle)
}

func runWithFactory(args []string, stdout, stderr io.Writer, factory supervisorFactory) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}

	switch args[0] {
	case "validate-config":
		return runValidateConfig(args[1:], stdout, stderr)
	case "run", "once":
		return runSupervisorWithFactory(args[0], args[1:], stdout, stderr, factory)
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "backup":
		return runBackup(args[1:], stdout, stderr)
	case "reconcile-public":
		return runReconcilePublicWithOpener(args[1:], stdout, stderr, openConfiguredRepository)
	case "divergence":
		return runDivergenceWithOpener(args[1:], stdout, stderr, openConfiguredRepository)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func runBackup(args []string, stdout, stderr io.Writer) int {
	configFile, destination, ok := backupArguments(args)
	if !ok {
		fmt.Fprintln(stderr, "usage: themachinewrites backup --config PATH --output PATH")
		return 2
	}
	cfg, err := config.LoadEnvy(configFile, os.Getenv)
	if err != nil {
		logging.New(stderr).Error("backup configuration failed", "error", err)
		return 1
	}
	runtime, err := store.Open(context.Background(), filepath.Join(cfg.RuntimeDir, "runtime.sqlite"))
	if err != nil {
		logging.New(stderr).Error("backup database open failed", "error", err)
		return 1
	}
	defer runtime.Close()
	if err := runtime.BackupTo(context.Background(), destination); err != nil {
		logging.New(stderr).Error("backup failed", "error", err)
		return 1
	}
	fmt.Fprintf(stdout, "backup complete: %s\n", destination)
	return 0
}

func backupArguments(args []string) (string, string, bool) {
	configFile, destination := "", ""
	for index := 0; index < len(args); index++ {
		if index+1 >= len(args) {
			return "", "", false
		}
		switch args[index] {
		case "--config":
			configFile = args[index+1]
		case "--output":
			destination = args[index+1]
		default:
			return "", "", false
		}
		index++
	}
	return configFile, destination, configFile != "" && destination != ""
}

func runReconcilePublic(args []string, stdout, stderr io.Writer) int {
	return runReconcilePublicWithOpener(args, stdout, stderr, openConfiguredRepository)
}

func runReconcilePublicWithOpener(args []string, stdout, stderr io.Writer, opener repositoryOpener) int {
	configFile, ok := configPath(args)
	if !ok {
		fmt.Fprintln(stderr, "usage: themachinewrites reconcile-public [--config PATH]")
		return 2
	}
	cfg, err := config.LoadEnvy(configFile, os.Getenv)
	if err != nil {
		logging.New(stderr).Error("reconciliation configuration failed", "error", err)
		return 1
	}
	repository, err := opener(context.Background(), cfg)
	if err != nil {
		logging.New(stderr).Error("reconciliation repository open failed", "error", err)
		return 1
	}
	runtime, err := store.Open(context.Background(), filepath.Join(cfg.RuntimeDir, "runtime.sqlite"))
	if err != nil {
		logging.New(stderr).Error("reconciliation database open failed", "error", err)
		return 1
	}
	defer runtime.Close()
	result, err := gitpub.ReconcilePublic(context.Background(), repository, runtime)
	if err != nil {
		logging.New(stderr).Error("publication reconciliation failed", "error", err)
		return 1
	}
	fmt.Fprintf(stdout, "reconciled %d publication(s) at %s\n", len(result.PostIDs), result.CommitSHA)
	return 0
}

func runDivergence(args []string, stdout, stderr io.Writer) int {
	return runDivergenceWithOpener(args, stdout, stderr, openConfiguredRepository)
}

func runDivergenceWithOpener(args []string, stdout, stderr io.Writer, opener repositoryOpener) int {
	configFile, ok := configPath(args)
	if !ok {
		fmt.Fprintln(stderr, "usage: themachinewrites divergence [--config PATH]")
		return 2
	}
	cfg, err := config.LoadEnvy(configFile, os.Getenv)
	if err != nil {
		logging.New(stderr).Error("divergence configuration failed", "error", err)
		return 1
	}
	repository, err := opener(context.Background(), cfg)
	if err != nil {
		logging.New(stderr).Error("divergence repository open failed", "error", err)
		return 1
	}
	runtime, err := store.Open(context.Background(), filepath.Join(cfg.RuntimeDir, "runtime.sqlite"))
	if err != nil {
		logging.New(stderr).Error("divergence database open failed", "error", err)
		return 1
	}
	defer runtime.Close()
	report, err := operations.InspectDivergence(context.Background(), runtime, repository)
	if err != nil {
		logging.New(stderr).Error("divergence report failed", "error", err)
		return 1
	}
	fmt.Fprint(stdout, report.String())
	if report.HasWarnings() {
		return 3
	}
	return 0
}

func runStatus(args []string, stdout, stderr io.Writer) int {
	path, ok := snapshotPath(args)
	if !ok {
		fmt.Fprintln(stderr, "usage: themachinewrites status [--snapshot PATH]")
		return 2
	}
	health, err := telemetry.ReadHealth(path)
	if err != nil {
		fmt.Fprintf(stderr, "unable to read health snapshot: %s\n", logging.RedactSecrets(err.Error()))
		return 1
	}
	fmt.Fprintf(stdout, "status: schema=%d\n", health.SchemaVersion)
	fmt.Fprintf(stdout, "last_poll: %s\n", displayTime(health.LastPoll))
	fmt.Fprintf(stdout, "last_model_call: %s\n", displayTime(health.LastModelCall))
	fmt.Fprintf(stdout, "last_confirmed_push: %s\n", displayTime(health.LastConfirmedPush))
	fmt.Fprintf(stdout, "last_error_kind: %s\n", valueOrNone(health.LastErrorKind))
	fmt.Fprintf(stdout, "paused_reason: %s\n", valueOrNone(health.PausedReason))
	return 0
}

func snapshotPath(args []string) (string, bool) {
	if len(args) == 0 {
		return "runtime/health.json", true
	}
	if len(args) == 2 && args[0] == "--snapshot" && args[1] != "" {
		return args[1], true
	}
	return "", false
}

func displayTime(value *time.Time) string {
	if value == nil {
		return "never"
	}
	return value.UTC().Format(time.RFC3339)
}

func valueOrNone(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

func runSupervisor(command string, args []string, stdout, stderr io.Writer) int {
	return runSupervisorWithFactory(command, args, stdout, stderr, buildSupervisorCycle)
}

func runSupervisorWithFactory(command string, args []string, stdout, stderr io.Writer, factory supervisorFactory) int {
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
	cycle, closeRuntime, err := factory(context.Background(), cfg, sourceConfig)
	if err != nil {
		logging.New(stderr).Error("supervisor runtime failed", "event", "supervisor.runtime", "error", err)
		return 1
	}
	defer closeRuntime()
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
