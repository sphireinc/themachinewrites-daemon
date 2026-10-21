package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/config"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/editorial"
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

With no command, run the daemon using --editorial-mode=random by default.
Editorial options for run and once:
  --editorial-mode=<mode>  choose a mode for each cycle (random) or pin one mode
  --list-editorial-modes   list every supported mode and description
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	return runWithFactory(args, stdout, stderr, buildSupervisorCycle)
}

func runWithFactory(args []string, stdout, stderr io.Writer, factory supervisorFactory) int {
	command, args, listModes, err := normalizeInvocation(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if listModes {
		writeEditorialModes(stdout)
		return 0
	}
	argsForCommand := args
	if command != "run" && command != "once" {
		argsForCommand = args
	}
	if command == "--help" || command == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}

	switch command {
	case "validate-config":
		return runValidateConfig(argsForCommand, stdout, stderr)
	case "run", "once":
		return runSupervisorWithFactory(command, argsForCommand, stdout, stderr, factory)
	case "status":
		return runStatus(argsForCommand, stdout, stderr)
	case "backup":
		return runBackup(argsForCommand, stdout, stderr)
	case "reconcile-public":
		return runReconcilePublicWithOpener(argsForCommand, stdout, stderr, openConfiguredRepository)
	case "divergence":
		return runDivergenceWithOpener(argsForCommand, stdout, stderr, openConfiguredRepository)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", command, usage)
		return 2
	}
}

func normalizeInvocation(args []string) (command string, commandArgs []string, listModes bool, err error) {
	if len(args) == 0 {
		return "run", nil, false, nil
	}
	if args[0] == "--list-editorial-modes" {
		if len(args) != 1 {
			return "", nil, false, errors.New("--list-editorial-modes does not accept additional arguments")
		}
		return "", nil, true, nil
	}
	command = args[0]
	known := map[string]bool{"run": true, "once": true, "validate-config": true, "status": true, "backup": true, "reconcile-public": true, "divergence": true}
	if !known[command] && (strings.HasPrefix(command, "--editorial-mode=") || command == "--editorial-mode" || command == "--config") {
		command = "run"
		commandArgs = args
	} else if known[command] || command == "--help" || command == "-h" {
		commandArgs = args[1:]
	} else {
		commandArgs = args[1:]
	}
	if _, _, err := parseEditorialModeFlag(commandArgs); err != nil {
		return "", nil, false, err
	}
	return command, commandArgs, false, nil
}

func parseEditorialModeFlag(args []string) (string, bool, error) {
	mode := ""
	for index := 0; index < len(args); index++ {
		var value string
		switch {
		case strings.HasPrefix(args[index], "--editorial-mode="):
			value = strings.TrimPrefix(args[index], "--editorial-mode=")
		case args[index] == "--editorial-mode":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
				return "", false, errors.New("--editorial-mode requires a value")
			}
			index++
			value = args[index]
		default:
			continue
		}
		if mode != "" {
			return "", false, errors.New("--editorial-mode may be specified only once")
		}
		if _, ok := editorial.Lookup(value); !ok {
			return "", false, fmt.Errorf("invalid editorial mode %q; valid modes: %s", value, strings.Join(editorial.ValidIDs(), ", "))
		}
		mode = value
	}
	return mode, mode != "", nil
}

func writeEditorialModes(writer io.Writer) {
	fmt.Fprintln(writer, "Supported editorial modes:")
	for _, id := range editorial.IDs() {
		mode, _ := editorial.Lookup(id)
		fmt.Fprintf(writer, "  %-20s %s\n", id, mode.Description)
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
	options, err := parseSupervisorOptions(args)
	if err != nil {
		fmt.Fprintf(stderr, "invalid %s arguments: %v\nusage: themachinewrites %s [--config PATH] [--editorial-mode=<mode>]\n", command, err, command)
		return 2
	}
	cfg, err := config.LoadEnvy(options.configPath, os.Getenv)
	if err != nil {
		logging.New(stderr).Error("supervisor configuration failed", "event", "supervisor.config", "error", err)
		return 1
	}
	if options.modeExplicit {
		cfg.Editorial.Mode = options.mode
	}
	if command == "once" {
		fmt.Fprintln(stdout, "once: configuration loaded")
	}
	sourceConfig, err := sources.LoadConfiguration(cfg.SourcesPath)
	if err != nil {
		logging.New(stderr).Error("source configuration failed", "event", "supervisor.sources", "error", err)
		return 1
	}
	if command == "once" {
		fmt.Fprintf(stdout, "once: source configuration loaded (enabled_sources=%d)\n", enabledCount(sourceConfig))
		fmt.Fprintln(stdout, "once: initializing runtime")
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
		fmt.Fprintf(stdout, "once: starting cycle (dry_run=%t)\n", cfg.DryRun)
		if err := adapter.Once(ctx); err != nil {
			fmt.Fprintln(stdout, "once: cycle failed; see stderr for stage details")
			logging.New(stderr).Error("supervisor cycle failed", "event", "supervisor.once", "error", err)
			return 1
		}
		fmt.Fprintln(stdout, "once: cycle completed")
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

type supervisorOptions struct {
	configPath   string
	mode         string
	modeExplicit bool
}

func parseSupervisorOptions(args []string) (supervisorOptions, error) {
	options := supervisorOptions{configPath: "config.yaml"}
	var err error
	options.mode, options.modeExplicit, err = parseEditorialModeFlag(args)
	if err != nil {
		return supervisorOptions{}, err
	}
	for index := 0; index < len(args); index++ {
		switch {
		case args[index] == "--config":
			if index+1 >= len(args) || args[index+1] == "" || strings.HasPrefix(args[index+1], "--") {
				return supervisorOptions{}, errors.New("--config requires a path")
			}
			options.configPath = args[index+1]
			index++
		case strings.HasPrefix(args[index], "--config="):
			options.configPath = strings.TrimPrefix(args[index], "--config=")
			if options.configPath == "" {
				return supervisorOptions{}, errors.New("--config requires a path")
			}
		case strings.HasPrefix(args[index], "--editorial-mode="):
			continue
		case args[index] == "--editorial-mode":
			index++
		case strings.HasPrefix(args[index], "-"):
			return supervisorOptions{}, fmt.Errorf("unknown option %q", args[index])
		default:
			return supervisorOptions{}, fmt.Errorf("unexpected argument %q", args[index])
		}
	}
	return options, nil
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
