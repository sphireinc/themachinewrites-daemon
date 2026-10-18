package config

import (
	"errors"
	"fmt"
)

const (
	gitPublishTokenEnv = "THEMACHINEWRITES_GIT_PUBLISH_TOKEN"
	researchTokenEnv   = "THEMACHINEWRITES_RESEARCH_TOKEN"
)

type Secret string

func (Secret) String() string {
	return "[REDACTED]"
}

func (s Secret) GoString() string {
	return "[REDACTED]"
}

func (s Secret) Reveal() string {
	return string(s)
}

type Secrets struct {
	GitPublishToken Secret
	ResearchToken   Secret
}

func LoadSecrets(getenv func(string) string) (Secrets, error) {
	if getenv == nil {
		return Secrets{}, errors.New("environment lookup is unavailable")
	}
	return Secrets{
		GitPublishToken: Secret(getenv(gitPublishTokenEnv)),
		ResearchToken:   Secret(getenv(researchTokenEnv)),
	}, nil
}

func (s Secrets) ValidateRequired() error {
	if s.GitPublishToken == "" {
		return fmt.Errorf("%s is required", gitPublishTokenEnv)
	}
	return nil
}

func (s Secrets) String() string {
	return "Secrets{GitPublishToken:[REDACTED], ResearchToken:[REDACTED]}"
}

func (s Secrets) GoString() string {
	return s.String()
}
