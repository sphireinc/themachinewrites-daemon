package config

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/editorial"
)

func (c Editorial) Validate() error {
	var problems []error
	if c.Mode != "" {
		if _, ok := editorial.Lookup(c.Mode); !ok {
			problems = append(problems, fmt.Errorf("editorial.mode %q is not registered", c.Mode))
		}
	}
	for id, weight := range c.Weights {
		mode, ok := editorial.Lookup(id)
		if !ok || !mode.RandomEligible {
			problems = append(problems, fmt.Errorf("editorial.weights contains non-random mode %q", id))
			continue
		}
		if weight < 0 || weight > 1_000_000 {
			problems = append(problems, fmt.Errorf("editorial weight for %q must be between 0 and 1000000", id))
		}
	}
	if c.HistoryLimit < 1 || c.HistoryLimit > 10000 {
		problems = append(problems, errors.New("editorial.history_limit must be between 1 and 10000"))
	}
	if c.ModeRepeatWindow < 0 || c.ModeRepeatWindow > 100 || c.TopicRepeatWindow < 0 || c.TopicRepeatWindow > 100 {
		problems = append(problems, errors.New("editorial repetition windows must be between 0 and 100"))
	}
	for name, value := range map[string]float64{
		"mode_repeat_penalty":  c.ModeRepeatPenalty,
		"topic_repeat_penalty": c.TopicRepeatPenalty,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
			problems = append(problems, fmt.Errorf("editorial.%s must be between 0 and 1", name))
		}
	}
	return errors.Join(problems...)
}

func (c Config) Validate() error {
	var problems []error
	if err := c.Editorial.Validate(); err != nil {
		problems = append(problems, err)
	}
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
	if strings.TrimSpace(c.CodexModel) == "" {
		problems = append(problems, errors.New("codex_model is required"))
	}
	if strings.TrimSpace(c.CodexHome) == "" || !filepath.IsAbs(c.CodexHome) {
		problems = append(problems, errors.New("codex_home must be an absolute dedicated directory"))
	} else if info, err := os.Lstat(c.CodexHome); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		problems = append(problems, errors.New("codex_home must be an existing real directory"))
	}
	if strings.TrimSpace(c.PublicBranch) == "" || strings.ContainsAny(c.PublicBranch, " \t\r\n") {
		problems = append(problems, errors.New("public_branch is required and must be a simple branch name"))
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
	parsedRemote, err := url.Parse(c.PublicRemote)
	if err != nil || parsedRemote.Scheme != "https" || parsedRemote.Hostname() == "" || parsedRemote.User != nil || parsedRemote.RawQuery != "" || parsedRemote.Fragment != "" {
		problems = append(problems, errors.New("public_remote must be a credential-free HTTPS URL"))
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
