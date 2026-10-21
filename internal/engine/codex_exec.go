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
	prefix          []string
	deniedReadPaths []string
}

func (r *commandProcessRunner) Run(ctx context.Context, executable string, args, env []string, stdin []byte) ([]byte, []byte, error) {
	commandName, commandArgs, err := commandInvocation(executable, append(append([]string(nil), r.prefix...), args...), r.deniedReadPaths)
	if err != nil {
		return nil, nil, err
	}
	command := exec.CommandContext(ctx, commandName, commandArgs...)
	command.Env = env
	command.Dir = workingDirectory(env)
	command.WaitDelay = 2 * time.Second
	command.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err = command.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

var sandboxExecutable = "sandbox-exec"

func commandInvocation(executable string, args, deniedReadPaths []string) (string, []string, error) {
	if len(deniedReadPaths) == 0 {
		return executable, args, nil
	}
	for _, path := range deniedReadPaths {
		if !filepath.IsAbs(path) {
			return "", nil, errors.New("sandbox denied read path must be absolute")
		}
		if _, err := filepath.EvalSymlinks(path); err != nil {
			return "", nil, errors.New("sandbox denied read path does not exist")
		}
	}
	profile, err := macOSSandboxProfile(deniedReadPaths)
	if err != nil {
		return "", nil, err
	}
	wrapped := []string{"-p", profile, executable}
	return sandboxExecutable, append(wrapped, args...), nil
}

func macOSSandboxProfile(paths []string) (string, error) {
	clauses := make([]string, 0, len(paths))
	for _, path := range paths {
		canonical, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", errors.New("sandbox denied read path does not exist")
		}
		path = canonical
		if strings.ContainsAny(path, "\"\r\n") {
			return "", errors.New("sandbox denied read path contains unsupported characters")
		}
		clauses = append(clauses, `(subpath "`+path+`")`)
	}
	return "(version 1)(allow default)(deny file-read-data " + strings.Join(clauses, " ") + ")", nil
}

type CodexExec struct {
	Executable      string
	Sandbox         string
	Timeout         time.Duration
	MaxOutputBytes  int
	Model           string
	Environment     []string
	Workspace       string
	SandboxRoot     string
	DeniedReadPaths []string
	Runner          processRunner
	TempDir         string
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
	if err := validateSandboxPaths(c.SandboxRoot, c.Workspace, c.TempDir); err != nil {
		return ExecOutput{}, err
	}
	environment, err := isolatedEnvironment(c.Environment, os.Environ(), filepath.Join(c.SandboxRoot, "home"), c.TempDir)
	if err != nil {
		return ExecOutput{}, err
	}
	schemaPath := filepath.Join(c.TempDir, "proposal.schema.json")
	outputPath := filepath.Join(c.TempDir, "last-message.json")
	if err := os.WriteFile(schemaPath, proposalSchema, 0o600); err != nil {
		return ExecOutput{}, fmt.Errorf("write proposal schema: %w", err)
	}
	defer os.Remove(outputPath)
	trusted := input.Instructions
	if input.EditorialMode != "" {
		trusted += "\n\nSELECTED EDITORIAL MODE: " + input.EditorialMode + "\n" + input.EditorialInstructions
	}
	prompt := "TRUSTED INSTRUCTIONS:\n" + trusted + "\n\nUNTRUSTED SOURCE DATA:\n" + input.Bundle + "\n"
	args := []string{
		"exec", "--sandbox", c.Sandbox, "--ephemeral", "--json",
		"--config", `approval_policy="never"`, "--config", `web_search="disabled"`,
		"--config", "sandbox_workspace_write.network_access=false",
		"--output-schema", schemaPath, "--output-last-message", outputPath,
		"--ignore-user-config", "--ignore-rules", "--skip-git-repo-check", "--cd", c.Workspace, "-",
	}
	if strings.TrimSpace(c.Model) != "" {
		args = append([]string{args[0], "--model", c.Model}, args[1:]...)
	}
	if commandRunner, ok := c.Runner.(*commandProcessRunner); ok && len(c.DeniedReadPaths) > 0 {
		commandRunner.deniedReadPaths = append([]string(nil), c.DeniedReadPaths...)
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
		return ExecOutput{}, fmt.Errorf("codex exec failed: %w (diagnostic=%s)", err, classifyCodexFailure(stderr))
	}
	lastMessage, readErr := os.ReadFile(outputPath)
	if readErr == nil && len(bytes.TrimSpace(lastMessage)) > 0 {
		stdout = lastMessage
	}
	return ExecOutput{Stdout: stdout, Stderr: stderr}, nil
}

func classifyCodexFailure(stderr []byte) string {
	message := strings.ToLower(string(stderr))
	switch {
	case strings.Contains(message, "sandbox_apply"), strings.Contains(message, "failed to apply sandbox"):
		return "native_sandbox_apply"
	case strings.Contains(message, "invalid_json_schema"), strings.Contains(message, "json schema"), strings.Contains(message, "schema"):
		return "structured_output_schema"
	case strings.Contains(message, "401"), strings.Contains(message, "403"), strings.Contains(message, "unauthorized"), strings.Contains(message, "not logged in"), strings.Contains(message, "authentication"):
		return "authentication"
	case strings.Contains(message, "sandbox"), strings.Contains(message, "permission denied"), strings.Contains(message, "operation not permitted"):
		return "sandbox_or_filesystem"
	case strings.Contains(message, "429"), strings.Contains(message, "rate limit"), strings.Contains(message, "quota"):
		return "rate_limit_or_quota"
	case strings.Contains(message, "resolve"), strings.Contains(message, "dns"), strings.Contains(message, "connection"), strings.Contains(message, "network"):
		return "network"
	default:
		return "unknown"
	}
}

func isolatedEnvironment(override, parent []string, home, tempDir string) ([]string, error) {
	source := parent
	if override != nil {
		source = override
	}
	values := make(map[string]string)
	for _, entry := range source {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !allowedCodexEnvironment[key] || key == "CODEX_HOME" {
			continue
		}
		if key != "HOME" && key != "TMPDIR" {
			values[key] = value
		}
	}
	values["HOME"] = home
	values["TMPDIR"] = tempDir
	if err := os.MkdirAll(home, 0o700); err != nil {
		return nil, fmt.Errorf("create isolated Codex home: %w", err)
	}
	if override != nil {
		for _, entry := range override {
			key, value, ok := strings.Cut(entry, "=")
			if !ok || !allowedCodexEnvironment[key] {
				return nil, errors.New("codex environment contains a variable outside the allowlist")
			}
			if key == "CODEX_HOME" {
				if err := requireDedicatedCodexHome(value); err != nil {
					return nil, err
				}
			}
			if key != "HOME" && key != "TMPDIR" {
				values[key] = value
			}
		}
	}
	keys := []string{"PATH", "USER", "LANG", "LC_ALL", "GOCACHE", "GOMODCACHE", "CODEX_HOME", "HOME", "TMPDIR"}
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		if value, ok := values[key]; ok {
			result = append(result, key+"="+value)
		}
	}
	return result, nil
}

func requireDedicatedCodexHome(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("CODEX_HOME must be an absolute dedicated directory")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("CODEX_HOME must be an existing real directory")
	}
	return nil
}

var allowedCodexEnvironment = map[string]bool{
	"PATH": true, "HOME": true, "USER": true, "TMPDIR": true, "LANG": true, "LC_ALL": true, "CODEX_HOME": true,
	"GOCACHE": true, "GOMODCACHE": true,
}

func validateSandboxPaths(root, workspace, tempDir string) error {
	home := filepath.Join(root, "home")
	if strings.TrimSpace(root) == "" || strings.TrimSpace(workspace) == "" || strings.TrimSpace(tempDir) == "" {
		return errors.New("isolated Codex root, workspace and temp directories are required")
	}
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return errors.New("isolated Codex root is invalid")
	}
	for name, path := range map[string]string{"workspace": workspace, "temp directory": tempDir, "home directory": home} {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("isolated Codex %s is invalid", name)
		}
		relative, err := filepath.Rel(cleanRoot, absolute)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("isolated Codex %s must be inside the sandbox root", name)
		}
		info, err := os.Stat(absolute)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("isolated Codex %s must be an existing directory", name)
		}
		if name == "workspace" {
			entries, err := os.ReadDir(absolute)
			if err != nil {
				return fmt.Errorf("inspect Codex workspace: %w", err)
			}
			if len(entries) != 0 {
				return errors.New("isolated Codex workspace must be empty")
			}
		}
	}
	return nil
}

func workingDirectory(environment []string) string {
	for _, entry := range environment {
		if value, ok := strings.CutPrefix(entry, "TMPDIR="); ok {
			return value
		}
	}
	return ""
}
