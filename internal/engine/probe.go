package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const MinimumCodexVersion = "0.163.0"

type commandRunner func(context.Context, string, ...string) ([]byte, []byte, error)

type Compatibility struct {
	Version      string
	SupportsExec bool
}

var codexVersionPattern = regexp.MustCompile(`(?m)(?:codex-cli\s+)?([0-9]+)\.([0-9]+)\.([0-9]+)`)

func Probe(ctx context.Context, executable string, runner commandRunner) (Compatibility, error) {
	if strings.TrimSpace(executable) == "" {
		return Compatibility{}, errors.New("codex executable is required")
	}
	if runner == nil {
		runner = runCommand
	}
	stdout, stderr, err := runner(ctx, executable, "--version")
	if err != nil {
		return Compatibility{}, fmt.Errorf("codex CLI is not available: %w: %s", err, bounded(string(stderr), 300))
	}
	version, err := parseVersion(string(stdout))
	if err != nil {
		return Compatibility{}, err
	}
	if compareVersions(version, MinimumCodexVersion) < 0 {
		return Compatibility{}, fmt.Errorf("codex CLI version %s is below minimum %s; run the documented upgrade check", version, MinimumCodexVersion)
	}
	help, securityErr, securityErrValue := runner(ctx, executable, "exec", "--help")
	if securityErrValue != nil {
		return Compatibility{}, fmt.Errorf("codex exec help probe failed: %w: %s", securityErrValue, bounded(string(securityErr), 300))
	}
	required := []string{"--output-schema", "--output-last-message", "--json", "--sandbox"}
	for _, flag := range required {
		if !bytes.Contains(help, []byte(flag)) {
			return Compatibility{}, fmt.Errorf("codex exec lacks required supported flag %s", flag)
		}
	}
	_, securityErr, securityErrValue = runner(ctx, executable, policyProbeArgs()...)
	if securityErrValue != nil {
		return Compatibility{}, fmt.Errorf("codex exec security policy probe failed: %w: %s", securityErrValue, bounded(string(securityErr), 300))
	}
	return Compatibility{Version: version, SupportsExec: true}, nil
}

func policyProbeArgs() []string {
	return []string{
		"exec", "--config", `approval_policy="never"`, "--config", `web_search="disabled"`,
		"--config", "sandbox_workspace_write.network_access=false", "--help",
	}
}

func parseVersion(output string) (string, error) {
	match := codexVersionPattern.FindStringSubmatch(output)
	if len(match) != 4 {
		return "", errors.New("could not parse codex CLI version")
	}
	return match[1] + "." + match[2] + "." + match[3], nil
}

func compareVersions(left, right string) int {
	render := func(value string) [3]int {
		var parts [3]int
		for index, part := range strings.Split(value, ".") {
			if index < len(parts) {
				parts[index], _ = strconv.Atoi(part)
			}
		}
		return parts
	}
	a, b := render(left), render(right)
	for index := range a {
		if a[index] < b[index] {
			return -1
		}
		if a[index] > b[index] {
			return 1
		}
	}
	return 0
}

func runCommand(ctx context.Context, executable string, args ...string) ([]byte, []byte, error) {
	command := exec.CommandContext(ctx, executable, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func bounded(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
