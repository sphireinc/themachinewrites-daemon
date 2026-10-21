package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadReadsTypedConfiguration(t *testing.T) {
	directory := t.TempDir()
	codexHome := filepath.Join(directory, "codex-home")
	if err := os.Mkdir(codexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "daemon.yaml")
	content := `poll_interval: 15m
sources_path: ./sources.yaml
runtime_dir: ./runtime
codex_executable: codex
codex_timeout: 8m
codex_model: gpt-6
codex_home: CODEX_HOME_PLACEHOLDER
public_repo_path: ./themachinewrites-public
public_branch: main
public_remote: https://github.com/example/public.git
budget:
  daily_model_invocations: 12
  daily_input_tokens: 120000
  daily_output_tokens: 24000
  max_attempts: 3
dry_run: true
`
	content = strings.ReplaceAll(content, "CODEX_HOME_PLACEHOLDER", codexHome)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Config{
		PollInterval:    15 * time.Minute,
		SourcesPath:     "./sources.yaml",
		RuntimeDir:      "./runtime",
		CodexExecutable: "codex",
		CodexTimeout:    8 * time.Minute,
		CodexModel:      "gpt-6",
		CodexHome:       codexHome,
		PublicRepoPath:  "./themachinewrites-public",
		PublicBranch:    "main",
		PublicRemote:    "https://github.com/example/public.git",
		Budget: Budget{
			DailyModelInvocations: 12,
			DailyInputTokens:      120000,
			DailyOutputTokens:     24000,
			MaxAttempts:           3,
		},
		Editorial: DefaultEditorial(),
		DryRun:    true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.yaml")
	if err := os.WriteFile(path, []byte("dry_run: true\nunknown_setting: enabled\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load() accepted an unknown key")
	}
}

func TestDefaultConfigurationHasNoCredentialsAndStartsDryRun(t *testing.T) {
	cfg := Default()
	if !cfg.DryRun {
		t.Fatal("Default().DryRun = false, want true")
	}
	if cfg.PollInterval <= 0 || cfg.CodexTimeout <= 0 {
		t.Fatalf("Default() durations = %v/%v, want positive", cfg.PollInterval, cfg.CodexTimeout)
	}
	dump, err := cfg.MarshalRedacted()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(dump), "quoted_api_key") {
		t.Fatalf("default configuration contains credential fields: %s", dump)
	}
}

func TestLoadEnvyEnvironmentOverridesConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.yaml")
	codexHome := filepath.Join(t.TempDir(), "codex-home")
	if err := os.Mkdir(codexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("dry_run: true\nsources_path: from-file\ncodex_model: gpt-6\ncodex_home: "+codexHome+"\npublic_branch: main\npublic_remote: https://github.com/example/public.git\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	getenv := func(name string) string {
		switch name {
		case "THEMACHINEWRITES_SOURCES_PATH":
			return "from-environment"
		case "THEMACHINEWRITES_PUBLIC_REPO_PATH":
			return "public-repository"
		}
		return ""
	}

	cfg, err := LoadEnvy(path, getenv)
	if err != nil {
		t.Fatalf("LoadEnvy() error = %v", err)
	}
	if cfg.SourcesPath != "from-environment" {
		t.Fatalf("SourcesPath = %q, want environment value", cfg.SourcesPath)
	}
	if !cfg.DryRun || cfg.PollInterval != Default().PollInterval {
		t.Fatalf("default fields were not preserved: %#v", cfg)
	}
}

func TestLoadEnvyRejectsInvalidYAMLWithoutSecretValues(t *testing.T) {
	const canary = "yaml-secret-canary"
	t.Setenv("THEMACHINEWRITES_GIT_PUBLISH_TOKEN", canary)
	path := filepath.Join(t.TempDir(), "daemon.yaml")
	if err := os.WriteFile(path, []byte("dry_run: true\nbad: [[\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadEnvy(path, os.Getenv)
	if err == nil {
		t.Fatal("LoadEnvy() accepted invalid YAML")
	}
	if strings.Contains(err.Error(), canary) {
		t.Fatal("YAML error leaked environment secret")
	}
}

func TestLoadEnvyRequiresRuntimeSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.yaml")
	if err := os.WriteFile(path, []byte("dry_run: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadEnvy(path, func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "public_repo_path") {
		t.Fatalf("LoadEnvy() error = %v, want public_repo_path", err)
	}
}

func TestValidateRejectsUnsafeConfiguration(t *testing.T) {
	directory := t.TempDir()
	sources := filepath.Join(directory, "sources.yaml")
	runtimeDirectory := filepath.Join(directory, "runtime")
	publicRepository := filepath.Join(directory, "public")
	codexHome := filepath.Join(directory, "codex-home")
	if err := os.WriteFile(sources, []byte("sources: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(runtimeDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(publicRepository, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(codexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	valid := Config{
		PollInterval:    10 * time.Minute,
		SourcesPath:     sources,
		RuntimeDir:      runtimeDirectory,
		CodexExecutable: "codex",
		CodexTimeout:    time.Minute,
		CodexModel:      "gpt-6",
		CodexHome:       codexHome,
		PublicRepoPath:  publicRepository,
		PublicBranch:    "main",
		PublicRemote:    "https://github.com/example/public.git",
		Budget: Budget{
			DailyModelInvocations: 10,
			DailyInputTokens:      100000,
			DailyOutputTokens:     20000,
			MaxAttempts:           3,
		},
		Editorial: DefaultEditorial(),
		DryRun:    true,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid configuration error = %v", err)
	}

	tests := []struct {
		name   string
		change func(*Config)
		want   string
	}{
		{name: "zero poll interval", change: func(c *Config) { c.PollInterval = 0 }, want: "poll_interval"},
		{name: "negative codex timeout", change: func(c *Config) { c.CodexTimeout = -time.Second }, want: "codex_timeout"},
		{name: "empty sources path", change: func(c *Config) { c.SourcesPath = "" }, want: "sources_path"},
		{name: "empty runtime dir", change: func(c *Config) { c.RuntimeDir = "" }, want: "runtime_dir"},
		{name: "empty codex executable", change: func(c *Config) { c.CodexExecutable = "  " }, want: "codex_executable"},
		{name: "empty codex model", change: func(c *Config) { c.CodexModel = "  " }, want: "codex_model"},
		{name: "empty codex home", change: func(c *Config) { c.CodexHome = "" }, want: "codex_home"},
		{name: "empty public repo", change: func(c *Config) { c.PublicRepoPath = "" }, want: "public_repo_path"},
		{name: "empty public branch", change: func(c *Config) { c.PublicBranch = "" }, want: "public_branch"},
		{name: "credential remote", change: func(c *Config) { c.PublicRemote = "https://token@example.org/public.git" }, want: "public_remote"},
		{name: "zero invocation budget", change: func(c *Config) { c.Budget.DailyModelInvocations = 0 }, want: "daily_model_invocations"},
		{name: "zero input budget", change: func(c *Config) { c.Budget.DailyInputTokens = 0 }, want: "daily_input_tokens"},
		{name: "zero output budget", change: func(c *Config) { c.Budget.DailyOutputTokens = 0 }, want: "daily_output_tokens"},
		{name: "unsafe retry count", change: func(c *Config) { c.Budget.MaxAttempts = 100 }, want: "max_attempts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.change(&cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want field %q", err, tt.want)
			}
		})
	}
}

func TestValidateFeedURLRequiresHTTPSExceptLocalFixtures(t *testing.T) {
	for _, tt := range []struct {
		raw         string
		allowLocal  bool
		shouldError bool
	}{
		{raw: "https://example.org/feed.xml"},
		{raw: "http://127.0.0.1:8080/feed.xml", allowLocal: true},
		{raw: "http://example.org/feed.xml", shouldError: true},
		{raw: "file:///etc/passwd", shouldError: true},
	} {
		parsed, parseErr := url.Parse(tt.raw)
		if parseErr != nil {
			t.Fatalf("url.Parse(%q): %v", tt.raw, parseErr)
		}
		err := validateFeedURL(parsed, tt.allowLocal)
		if (err != nil) != tt.shouldError {
			t.Errorf("validateFeedURL(%q, %v) error = %v, shouldError = %v", tt.raw, tt.allowLocal, err, tt.shouldError)
		}
	}
}

func TestSecretsLoadOnlyFromEnvironment(t *testing.T) {
	const canary = "canary-publish-credential"
	t.Setenv("THEMACHINEWRITES_GIT_PUBLISH_TOKEN", canary)

	secrets, err := LoadSecrets(os.Getenv)
	if err != nil {
		t.Fatalf("LoadSecrets() error = %v", err)
	}
	if got := secrets.GitPublishToken.Reveal(); got != canary {
		t.Fatalf("GitPublishToken = %q, want canary", got)
	}
	if strings.Contains(secrets.String(), canary) || strings.Contains(fmt.Sprint(secrets), canary) {
		t.Fatal("formatted secrets leaked the credential")
	}

	cfg := Config{
		SourcesPath:     "sources.yaml",
		RuntimeDir:      "runtime",
		CodexExecutable: "codex",
		PublicRepoPath:  "public",
	}
	dump, err := cfg.MarshalRedacted()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(dump), canary) {
		t.Fatal("configuration dump leaked the credential")
	}
}

func TestLoadSecretsDoesNotPutCredentialInValidationError(t *testing.T) {
	const canary = "do-not-print-this-token"
	t.Setenv("THEMACHINEWRITES_GIT_PUBLISH_TOKEN", canary)
	t.Setenv("THEMACHINEWRITES_RESEARCH_TOKEN", "research-canary")

	secrets, err := LoadSecrets(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(errString(secrets.ValidateRequired()), canary) {
		t.Fatal("secret validation error leaked the publish credential")
	}
}

func TestDryRunDoesNotRequirePublishCredential(t *testing.T) {
	secrets, err := LoadSecrets(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if err := secrets.ValidateForPublication(false); err != nil {
		t.Fatalf("dry-run validation error = %v", err)
	}
	if err := secrets.ValidateForPublication(true); err == nil {
		t.Fatal("live publication accepted a missing publish credential")
	}
}

func TestLiveModeRequiresAbsolutePublicRepositoryPath(t *testing.T) {
	cfg := Default()
	cfg.DryRun = false
	cfg.PublicRepoPath = "./themachinewrites-public"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "public_repo_path must be absolute") {
		t.Fatalf("Validate() error = %v, want absolute public_repo_path rejection", err)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
