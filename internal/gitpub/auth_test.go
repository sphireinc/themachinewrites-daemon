package gitpub

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
)

func TestAuthenticatedGitEnvironmentUsesEphemeralScopedHeader(t *testing.T) {
	base := []string{"PATH=/usr/bin:/bin", "HOME=/private/tmp/core"}
	environment := authenticatedGitEnvironment(base, "scoped-token-canary")
	joined := strings.Join(environment, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte("x-access-token:scoped-token-canary"))
	for _, required := range []string{
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=http.https://github.com/.extraheader",
		"GIT_CONFIG_VALUE_0=AUTHORIZATION: basic " + encoded,
		"PATH=/usr/bin:/bin",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("authenticated environment missing %q: %#v", required, environment)
		}
	}
	if strings.Contains(joined, "scoped-token-canary") {
		t.Fatal("raw token retained in Git environment")
	}
}

func TestAuthenticatedRepositoryRejectsEmptyToken(t *testing.T) {
	if _, err := AuthenticatedRepository(context.Background(), RepositoryConfig{}, ""); err == nil {
		t.Fatal("empty publisher token accepted")
	}
}

func TestAuthenticatedRepositoryInjectsCredentialOnlyIntoGitProcess(t *testing.T) {
	if _, err := AuthenticatedRepository(context.Background(), RepositoryConfig{}, "token"); err == nil {
		t.Fatal("invalid repository config accepted")
	}
	t.Setenv("THEMACHINEWRITES_GIT_PUBLISH_TOKEN", "forbidden-parent-token")
	environment := authenticatedGitEnvironment([]string{"PATH=/usr/bin"}, "scoped-token-canary")
	if strings.Contains(strings.Join(environment, "\n"), "THEMACHINEWRITES_GIT_PUBLISH_TOKEN") {
		t.Fatal("publisher token name leaked into Git environment")
	}
}
