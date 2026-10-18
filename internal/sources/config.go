package sources

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Configuration struct {
	Sources []Source `yaml:"sources"`
}

type Source struct {
	ID              string        `yaml:"id"`
	URL             *URL          `yaml:"url"`
	Enabled         bool          `yaml:"enabled"`
	RequestInterval time.Duration `yaml:"request_interval"`
	Name            string        `yaml:"name"`
	Category        string        `yaml:"category,omitempty"`
}

type URL struct {
	value *url.URL
}

func (u *URL) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode || strings.TrimSpace(node.Value) == "" {
		return errors.New("url must be a nonempty scalar")
	}
	parsed, err := url.Parse(node.Value)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	u.value = parsed
	return nil
}

func (u *URL) String() string {
	if u == nil || u.value == nil {
		return ""
	}
	return u.value.String()
}

func (u *URL) Parsed() *url.URL {
	if u == nil {
		return nil
	}
	return u.value
}

func LoadConfiguration(path string) (Configuration, error) {
	file, err := os.Open(path)
	if err != nil {
		return Configuration{}, fmt.Errorf("open source configuration: %w", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var config Configuration
	if err := decoder.Decode(&config); err != nil {
		return Configuration{}, fmt.Errorf("decode source configuration: %w", err)
	}
	if err := config.Validate(); err != nil {
		return Configuration{}, err
	}
	return config, nil
}

func (c Configuration) Validate() error {
	if len(c.Sources) == 0 {
		return errors.New("source configuration must include at least one source")
	}
	seen := make(map[string]struct{}, len(c.Sources))
	var problems []error
	for index, source := range c.Sources {
		if _, exists := seen[source.ID]; exists {
			problems = append(problems, fmt.Errorf("duplicate source id %q", source.ID))
		}
		seen[source.ID] = struct{}{}
		if err := source.Validate(); err != nil {
			problems = append(problems, fmt.Errorf("source %d: %w", index, err))
		}
	}
	return errors.Join(problems...)
}

func (s Source) Validate() error {
	var problems []error
	if strings.TrimSpace(s.ID) == "" {
		problems = append(problems, errors.New("id is required"))
	}
	if strings.TrimSpace(s.Name) == "" {
		problems = append(problems, errors.New("name is required"))
	}
	parsed := s.URL.Parsed()
	if parsed == nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		problems = append(problems, errors.New("url must be an absolute HTTPS URL"))
	} else if parsed.User != nil {
		problems = append(problems, errors.New("url must not contain credentials"))
	}
	if s.RequestInterval <= 0 {
		problems = append(problems, errors.New("request_interval must be greater than zero"))
	}
	return errors.Join(problems...)
}
