package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func (c Config) Validate() error {
	var problems []error
	if c.PollInterval <= 0 {
		problems = append(problems, errors.New("poll_interval must be greater than zero"))
	}
	if c.CodexTimeout <= 0 {
		problems = append(problems, errors.New("codex_timeout must be greater than zero"))
	}
	for name, value := range map[string]string{
		"sources_path":     c.SourcesPath,
		"runtime_dir":      c.RuntimeDir,
		"public_repo_path": c.PublicRepoPath,
	} {
		if strings.TrimSpace(value) == "" {
			problems = append(problems, fmt.Errorf("%s is required", name))
			continue
		}
		if filepath.IsAbs(value) {
			if _, err := os.Stat(value); err != nil {
				problems = append(problems, fmt.Errorf("%s is not readable: %w", name, err))
			}
		}
	}
	if strings.TrimSpace(c.CodexExecutable) == "" {
		problems = append(problems, errors.New("codex_executable is required"))
	}
	if c.Budget.DailyModelInvocations <= 0 {
		problems = append(problems, errors.New("budget.daily_model_invocations must be greater than zero"))
	}
	if c.Budget.DailyInputTokens <= 0 {
		problems = append(problems, errors.New("budget.daily_input_tokens must be greater than zero"))
	}
	if c.Budget.DailyOutputTokens <= 0 {
		problems = append(problems, errors.New("budget.daily_output_tokens must be greater than zero"))
	}
	if c.Budget.MaxAttempts < 1 || c.Budget.MaxAttempts > 10 {
		problems = append(problems, errors.New("budget.max_attempts must be between 1 and 10"))
	}
	if !c.DryRun && !filepath.IsAbs(c.PublicRepoPath) {
		problems = append(problems, errors.New("public_repo_path must be absolute when dry_run is false"))
	}
	return errors.Join(problems...)
}

func validateFeedURL(parsed *url.URL, allowLocal bool) error {
	if parsed.Scheme == "https" && parsed.Hostname() != "" {
		return nil
	}
	if allowLocal && parsed.Scheme == "http" && (parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "localhost") {
		return nil
	}
	return errors.New("feed URL must use HTTPS; HTTP is allowed only for explicit local test fixtures")
}
