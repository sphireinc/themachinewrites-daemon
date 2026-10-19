package engine

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FactoryConfig struct {
	Executable      string
	Model           string
	Timeout         time.Duration
	SandboxRoot     string
	CodexHome       string
	DeniedReadPaths []string
}

func NewBoundedCodexEngine(config FactoryConfig) (CodexEngine, error) {
	if strings.TrimSpace(config.Executable) == "" || strings.TrimSpace(config.Model) == "" || config.Timeout <= 0 || strings.TrimSpace(config.SandboxRoot) == "" {
		return CodexEngine{}, errors.New("codex executable, model, timeout and sandbox root are required")
	}
	root, err := filepath.Abs(config.SandboxRoot)
	if err != nil {
		return CodexEngine{}, errors.New("codex sandbox root is invalid")
	}
	for _, directory := range []string{"workspace", "tmp", "home"} {
		path := filepath.Join(root, directory)
		entries, readErr := os.ReadDir(path)
		if readErr == nil && len(entries) > 0 {
			return CodexEngine{}, errors.New("codex sandbox directories must start empty")
		}
		if err := os.MkdirAll(path, 0o700); err != nil {
			return CodexEngine{}, err
		}
	}
	environment := []string{"PATH=" + os.Getenv("PATH")}
	for _, key := range []string{"USER", "LANG", "LC_ALL", "GOCACHE", "GOMODCACHE"} {
		if value := os.Getenv(key); value != "" {
			environment = append(environment, key+"="+value)
		}
	}
	if strings.TrimSpace(config.CodexHome) != "" {
		environment = append(environment, "CODEX_HOME="+config.CodexHome)
	}
	return CodexEngine{Exec: CodexExec{
		Executable: config.Executable, Model: config.Model, Sandbox: "read-only", Timeout: config.Timeout,
		MaxOutputBytes: 64 * 1024, Environment: environment, SandboxRoot: root,
		Workspace: filepath.Join(root, "workspace"), TempDir: filepath.Join(root, "tmp"),
		DeniedReadPaths: append([]string(nil), config.DeniedReadPaths...), Runner: &commandProcessRunner{},
	}}, nil
}
