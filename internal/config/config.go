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
