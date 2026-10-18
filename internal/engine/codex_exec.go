package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type processRunner interface {
	Run(context.Context, string, []string, []string, []byte) ([]byte, []byte, error)
}

type commandProcessRunner struct {
	prefix []string
}

func (r *commandProcessRunner) Run(ctx context.Context, executable string, args, env []string, stdin []byte) ([]byte, []byte, error) {
	commandArgs := append(append([]string(nil), r.prefix...), args...)
	command := exec.CommandContext(ctx, executable, commandArgs...)
	command.Env = env
	command.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

type CodexExec struct {
	Executable     string
	Sandbox        string
	Timeout        time.Duration
	MaxOutputBytes int
	Environment    []string
	Runner         processRunner
	TempDir        string
}

type ExecOutput struct {
	Stdout []byte
	Stderr []byte
}

func (c CodexExec) Run(ctx context.Context, input Input) (ExecOutput, error) {
	if strings.TrimSpace(c.Executable) == "" {
		return ExecOutput{}, errors.New("codex executable is required")
	}
	if c.Sandbox != "read-only" {
		return ExecOutput{}, errors.New("codex sandbox must be read-only")
	}
	if c.Timeout <= 0 || c.MaxOutputBytes <= 0 || c.Runner == nil {
		return ExecOutput{}, errors.New("codex execution limits are not configured")
	}
	environment := c.Environment
	if environment == nil {
		environment = minimalEnvironment(os.Environ())
	}
	for _, entry := range environment {
		key, _, _ := strings.Cut(entry, "=")
		if strings.Contains(strings.ToUpper(key), "TOKEN") || strings.Contains(strings.ToUpper(key), "SECRET") {
			return ExecOutput{}, errors.New("codex environment contains forbidden credential variable")
		}
	}
	tempDir := c.TempDir
	if tempDir == "" {
		var err error
		tempDir, err = os.MkdirTemp("", "themachinewrites-codex-")
		if err != nil {
			return ExecOutput{}, fmt.Errorf("create codex temp dir: %w", err)
		}
		defer os.RemoveAll(tempDir)
	}
	schemaPath := filepath.Join(tempDir, "proposal.schema.json")
	outputPath := filepath.Join(tempDir, "last-message.json")
	if err := os.WriteFile(schemaPath, proposalSchema, 0o600); err != nil {
		return ExecOutput{}, fmt.Errorf("write proposal schema: %w", err)
	}
	defer os.Remove(outputPath)
	prompt := "TRUSTED INSTRUCTIONS:\n" + input.Instructions + "\n\nUNTRUSTED SOURCE DATA:\n" + input.Bundle + "\n"
	args := []string{
		"exec", "--sandbox", c.Sandbox, "--ephemeral", "--json",
		"--output-schema", schemaPath, "--output-last-message", outputPath,
		"--ignore-user-config", "--ignore-rules", "--skip-git-repo-check", "-",
	}
	processCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	stdout, stderr, err := c.Runner.Run(processCtx, c.Executable, args, environment, []byte(prompt))
	if len(stdout) > c.MaxOutputBytes || len(stderr) > c.MaxOutputBytes {
		return ExecOutput{}, errors.New("codex output exceeded configured limit")
	}
	if err != nil {
		if errors.Is(processCtx.Err(), context.DeadlineExceeded) {
			return ExecOutput{}, context.DeadlineExceeded
		}
		return ExecOutput{}, fmt.Errorf("codex exec failed: %w", err)
	}
	lastMessage, readErr := os.ReadFile(outputPath)
	if readErr == nil && len(bytes.TrimSpace(lastMessage)) > 0 {
		stdout = lastMessage
	}
	return ExecOutput{Stdout: stdout, Stderr: stderr}, nil
}

func minimalEnvironment(environment []string) []string {
	allowed := map[string]bool{
		"PATH": true, "HOME": true, "USER": true, "TMPDIR": true, "LANG": true, "LC_ALL": true,
		"GOCACHE": true, "GOMODCACHE": true,
	}
	var result []string
	for _, entry := range environment {
		key, _, _ := strings.Cut(entry, "=")
		if allowed[key] {
			result = append(result, entry)
		}
	}
	return result
}
