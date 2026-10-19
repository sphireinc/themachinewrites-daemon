package publication

import (
	"testing"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
)

func TestValidateCitationsRejectsUnsafeAndMismatchedSources(t *testing.T) {
	base := validProposal()
	if err := ValidateCitations(base); err != nil {
		t.Fatalf("valid citations error = %v", err)
	}
	tests := []struct {
		name string
		url  string
	}{
		{name: "HTTP", url: "http://example.org/source"},
		{name: "metadata IP", url: "https://169.254.169.254/latest/meta-data"},
		{name: "credentials", url: "https://user:pass@example.org/source"},
		{name: "file scheme", url: "file:///etc/passwd"},
		{name: "malformed", url: "https://[::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proposal := base
			proposal.Sources = append([]engine.Source(nil), base.Sources...)
			proposal.Sources[0].URL = tt.url
			if err := ValidateCitations(proposal); err == nil {
				t.Fatalf("unsafe citation %q accepted", tt.url)
			}
		})
	}
	missing := base
	missing.Sources = nil
	if err := ValidateCitations(missing); err == nil {
		t.Fatal("missing citations accepted")
	}
	duplicate := base
	duplicate.Sources = append([]engine.Source(nil), base.Sources...)
	duplicate.Sources = append(duplicate.Sources, engine.Source{
		Title: "Syndicated copy", URL: "https://example.org/source?utm_source=feed",
		RetrievedAt: "2026-10-18T20:01:00Z",
	})
	if err := ValidateCitations(duplicate); err == nil {
		t.Fatal("tracking variant duplicate accepted")
	}
}
