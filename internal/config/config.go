package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	PollInterval    time.Duration `yaml:"poll_interval"`
	SourcesPath     string        `yaml:"sources_path"`
	RuntimeDir      string        `yaml:"runtime_dir"`
	CodexExecutable string        `yaml:"codex_executable"`
	CodexTimeout    time.Duration `yaml:"codex_timeout"`
	PublicRepoPath  string        `yaml:"public_repo_path"`
	Budget          Budget        `yaml:"budget"`
	DryRun          bool          `yaml:"dry_run"`
}

type Budget struct {
	DailyModelInvocations int `yaml:"daily_model_invocations"`
	DailyInputTokens      int `yaml:"daily_input_tokens"`
	DailyOutputTokens     int `yaml:"daily_output_tokens"`
	MaxAttempts           int `yaml:"max_attempts"`
}

func Default() Config {
	return Config{
		PollInterval:    15 * time.Minute,
		SourcesPath:     "./sources.yaml",
		RuntimeDir:      "./runtime",
		CodexExecutable: "codex",
		CodexTimeout:    10 * time.Minute,
		PublicRepoPath:  "",
		Budget: Budget{
			DailyModelInvocations: 8,
			DailyInputTokens:      200000,
			DailyOutputTokens:     40000,
			MaxAttempts:           3,
		},
		DryRun: true,
	}
}

func LoadEnvy(path string, getenv func(string) string) (Config, error) {
	cfg := Default()
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open configuration: %w", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode configuration: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return Config{}, fmt.Errorf("decode trailing configuration: %w", err)
		}
		return Config{}, errors.New("configuration must contain exactly one YAML document")
	}
	if getenv == nil {
		return Config{}, errors.New("environment lookup is unavailable")
	}
	applyEnvironment(&cfg, getenv)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyEnvironment(cfg *Config, getenv func(string) string) {
	for name, target := range map[string]*string{
		"THEMACHINEWRITES_SOURCES_PATH":     &cfg.SourcesPath,
		"THEMACHINEWRITES_RUNTIME_DIR":      &cfg.RuntimeDir,
		"THEMACHINEWRITES_CODEX_EXECUTABLE": &cfg.CodexExecutable,
		"THEMACHINEWRITES_PUBLIC_REPO_PATH": &cfg.PublicRepoPath,
	} {
		if value := getenv(name); value != "" {
			*target = value
		}
	}
}

func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open configuration: %w", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode configuration: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return Config{}, fmt.Errorf("decode trailing configuration: %w", err)
		}
		return Config{}, errors.New("configuration must contain exactly one YAML document")
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) MarshalRedacted() ([]byte, error) {
	return yaml.Marshal(c)
}
