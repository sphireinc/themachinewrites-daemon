package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type editorialFakeEngine struct {
	proposal Proposal
	err      error
	inputs   []Input
}

func (f *editorialFakeEngine) Run(_ context.Context, input Input) (Proposal, error) {
	f.inputs = append(f.inputs, input)
	return f.proposal, f.err
}

func TestRepeatedDeferLeavesRepositoryUntouched(t *testing.T) {
	public := t.TempDir()
	draft := filepath.Join(public, "feed", "latest.json")
	if err := os.MkdirAll(filepath.Dir(draft), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(draft, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	engine := &editorialFakeEngine{proposal: Proposal{Action: ActionDefer, Reason: "Nothing meaningful to add."}}
	for range 3 {
		decision, err := Decide(context.Background(), engine, Input{Instructions: "evaluate", Bundle: "{}"}, public)
		if err != nil {
			t.Fatal(err)
		}
		if decision.ContentCandidate != nil || decision.PublicWrite {
			t.Fatalf("defer decision = %#v", decision)
		}
	}
	content, err := os.ReadFile(draft)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "unchanged" {
		t.Fatalf("public file changed to %q", content)
	}
	if len(engine.inputs) != 3 {
		t.Fatalf("engine calls = %d, want 3", len(engine.inputs))
	}
}

func TestPublishEntersValidationWithoutPublicWrite(t *testing.T) {
	engine := &editorialFakeEngine{proposal: Proposal{
		Action: ActionPublish, Reason: "Evidence warrants publication.",
		Title: "A careful observation", Type: "observation", Body: "Grounded analysis.",
		Sources: []Source{{Title: "Source", URL: "https://example.org/source", RetrievedAt: "2026-10-18T20:00:00Z"}},
		Topics:  []string{"technology"}, ConfidenceRationale: "Source supports the claim.",
	}}
	public := t.TempDir()
	decision, err := Decide(context.Background(), engine, Input{Instructions: "evaluate", Bundle: "{}"}, public)
	if err != nil {
		t.Fatal(err)
	}
	if decision.ContentCandidate == nil || decision.PublicWrite {
		t.Fatalf("publish decision = %#v, want validation candidate without public write", decision)
	}
	entries, err := os.ReadDir(public)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("publish decision wrote directly to public repo: %#v", entries)
	}
}

func TestEngineFailureDoesNotFallbackToPublishing(t *testing.T) {
	engine := &editorialFakeEngine{err: errors.New("engine unavailable")}
	if _, err := Decide(context.Background(), engine, Input{Instructions: "evaluate", Bundle: "{}"}, t.TempDir()); err == nil {
		t.Fatal("engine failure was converted into a decision")
	}
}
