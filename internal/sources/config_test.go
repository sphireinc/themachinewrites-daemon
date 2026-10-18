package sources

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigurationParsesEnabledSourceAllowlist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sources.yaml")
	content := `sources:
  - id: primary-news
    url: https://example.org/feed.xml
    enabled: true
    request_interval: 15m
    name: Primary News
    category: technology
  - id: disabled-feed
    url: https://example.net/feed.xml
    enabled: false
    request_interval: 1h
    name: Disabled Feed
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfiguration(path)
	if err != nil {
		t.Fatalf("LoadConfiguration() error = %v", err)
	}
	if len(config.Sources) != 2 {
		t.Fatalf("source count = %d, want 2", len(config.Sources))
	}
	first := config.Sources[0]
	if first.ID != "primary-news" || first.Name != "Primary News" ||
		first.URL.String() != "https://example.org/feed.xml" ||
		first.RequestInterval != 15*time.Minute || first.Category != "technology" || !first.Enabled {
		t.Fatalf("first source = %#v", first)
	}
}

func TestConfigurationRejectsDuplicateIDsInvalidSchemesAndIntervals(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "duplicate IDs",
			content: `sources:
  - {id: duplicate, url: https://example.org/a, enabled: true, request_interval: 15m, name: A}
  - {id: duplicate, url: https://example.org/b, enabled: true, request_interval: 15m, name: B}`,
			want: "duplicate source id",
		},
		{
			name: "invalid scheme",
			content: `sources:
  - {id: file-feed, url: file:///etc/passwd, enabled: true, request_interval: 15m, name: File}`,
			want: "HTTPS",
		},
		{
			name: "nonpositive interval",
			content: `sources:
  - {id: bad-interval, url: https://example.org/feed, enabled: true, request_interval: 0s, name: Bad}`,
			want: "request_interval",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sources.yaml")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadConfiguration(path)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.want)) {
				t.Fatalf("LoadConfiguration() error = %v, want %q", err, tt.want)
			}
		})
	}
}
