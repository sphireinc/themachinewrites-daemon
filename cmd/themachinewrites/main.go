package main

import (
	"fmt"
	"io"
	"os"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/config"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/logging"
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
		fmt.Fprintf(stdout, "%s command not implemented\n", args[0])
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
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
