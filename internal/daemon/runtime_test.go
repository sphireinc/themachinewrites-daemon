package daemon

import (
	"context"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/memory"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/observations"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/publication"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/sources"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/supervisor"
	"gopkg.in/yaml.v3"
)

type fakeFetcher struct{}

func (fakeFetcher) Fetch(context.Context, *url.URL, sources.Cursor) (sources.FetchResult, error) {
	return sources.FetchResult{Body: []byte(`<rss><channel><item><guid>one</guid><title>Fixture headline</title><link>https://example.org/one</link><pubDate>Sun, 18 Oct 2026 20:00:00 GMT</pubDate><description>Fixture source excerpt.</description></item></channel></rss>`)}, nil
}

type deferEngine struct{}

func (deferEngine) Run(context.Context, engine.Input) (engine.Proposal, error) {
	return engine.Proposal{Action: engine.ActionDefer, Reason: "No contribution."}, nil
}

type captureEngine struct{ input engine.Input }

func (e *captureEngine) Run(_ context.Context, input engine.Input) (engine.Proposal, error) {
	e.input = input
	return engine.Proposal{Action: engine.ActionDefer, Reason: "Captured."}, nil
}

func TestRuntimePollsIngestsAndDefersInDryRun(t *testing.T) {
	ctx := context.Background()
	runtime, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	sourceURL := &sources.URL{}
	encoded := "https://example.org/feed.xml"
	if err := sourceURL.UnmarshalYAML(textNode(encoded)); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 18, 20, 1, 0, 0, time.UTC)
	daemon := Runtime{
		Sources: sources.Configuration{Sources: []sources.Source{{ID: "fixture", Name: "Fixture", Enabled: true, RequestInterval: time.Minute, URL: sourceURL}}},
		Fetcher: fakeFetcher{}, Store: runtime, Engine: deferEngine{}, PublicRoot: t.TempDir(), DryRun: true,
		Now: func() time.Time { return now },
	}
	cycle := supervisor.Cycle{Poll: daemon.Poll, Decide: daemon.DecideAndPublish, DryRun: true}
	if err := cycle.Once(ctx); err != nil {
		t.Fatal(err)
	}
	candidates, err := runtime.PendingCandidates(ctx, 10)
	if err != nil || len(candidates) != 0 {
		t.Fatalf("dry-run should retain no evaluable candidate after defer: %#v, %v", candidates, err)
	}
	var state, headline string
	if err := runtime.DB().QueryRowContext(ctx, "SELECT decision_state, headline FROM observed_items LIMIT 1").Scan(&state, &headline); err != nil {
		t.Fatal(err)
	}
	if state != "deferred" || headline != "Fixture headline" {
		t.Fatalf("candidate state/headline = %s/%s", state, headline)
	}
}

func TestRuntimeRejectsIncompleteDependencies(t *testing.T) {
	var daemon Runtime
	if errs := daemon.Poll(context.Background()); len(errs) == 0 || !strings.Contains(errs[0].Error(), "incomplete") {
		t.Fatalf("Poll errors = %v", errs)
	}
	if err := daemon.DecideAndPublish(context.Background()); err == nil {
		t.Fatal("incomplete runtime accepted")
	}
	_ = slog.Default()
}

func TestRuntimeDeferConsumesCandidateAcrossCycles(t *testing.T) {
	ctx := context.Background()
	runtime, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := observations.Ingest(ctx, runtime, observations.Candidate{
		URL: "https://example.org/one", Headline: "Fixture headline", SourceID: "fixture", SourceName: "Fixture",
		PublishedAt: time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC), RetrievedAt: time.Date(2026, 10, 18, 20, 1, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	daemon := Runtime{Store: runtime, Engine: deferEngine{}, PublicRoot: t.TempDir(), Now: func() time.Time { return time.Date(2026, 10, 18, 20, 2, 0, 0, time.UTC) }}
	if err := daemon.DecideAndPublish(ctx); err != nil {
		t.Fatal(err)
	}
	candidates, err := runtime.PendingCandidates(ctx, 10)
	if err != nil || len(candidates) != 0 {
		t.Fatalf("deferred candidate remains pending: %#v, %v", candidates, err)
	}
}

func TestRuntimeSuppliesRestartedMemoryToDecisionBundle(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "runtime.sqlite")
	runtime, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	if err := memory.Write(ctx, runtime, memory.Memory{
		ID: "technology-prior", Kind: memory.KindObservation, Summary: "Prior technology conclusion.",
		EvidenceLinks: []string{"https://example.org/prior"}, RelatedPostIDs: []string{"01M589K0G00000000000000000"},
		UpdatedAt: at, SourceHash: "hash-prior",
	}); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Close(); err != nil {
		t.Fatal(err)
	}
	runtime, err = store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := observations.Ingest(ctx, runtime, observations.Candidate{
		URL: "https://example.org/new", Headline: "Technology development", SourceID: "fixture", SourceName: "Fixture", Category: "technology",
		PublishedAt: at, RetrievedAt: at.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	capture := &captureEngine{}
	daemon := Runtime{Store: runtime, Engine: capture, PublicRoot: t.TempDir(), Now: func() time.Time { return at.Add(time.Hour) }}
	if err := daemon.DecideAndPublish(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(capture.input.Bundle, "technology-prior") || !strings.Contains(capture.input.Bundle, "Prior technology conclusion.") {
		t.Fatalf("bundle omitted restarted memory: %s", capture.input.Bundle)
	}
}

func TestRuntimeSuppliesMaturedPredictionReviewContext(t *testing.T) {
	ctx := context.Background()
	runtime, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	now := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	if _, err := observations.Ingest(ctx, runtime, observations.Candidate{
		URL: "https://example.org/new", Headline: "A new development", SourceID: "fixture", SourceName: "Fixture", Category: "technology",
		PublishedAt: now, RetrievedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	publicRoot := t.TempDir()
	postedAt := now.Add(-40 * 24 * time.Hour)
	postID, err := publication.GenerateID("mature-prediction", postedAt)
	if err != nil {
		t.Fatal(err)
	}
	post, err := publication.Render(engine.Proposal{
		Action: engine.ActionPublish, Reason: "fixture", Title: "A mature prediction", Type: "prediction", Body: "A prediction to revisit.",
		Sources: []engine.Source{{Title: "Fixture", URL: "https://example.org/prediction", RetrievedAt: now.Add(-40 * 24 * time.Hour).Format(time.RFC3339)}},
		Topics:  []string{"technology"}, RelatedIDs: []string{}, ConfidenceRationale: "fixture",
	}, publication.RenderInput{ID: postID, PublishedAt: postedAt, ModelIdentifier: "fixture", PromptVersion: "p1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := publication.WriteRenderedPost(publicRoot, post); err != nil {
		t.Fatal(err)
	}
	capture := &captureEngine{}
	daemon := Runtime{Store: runtime, Engine: capture, PublicRoot: publicRoot, Now: func() time.Time { return now }}
	if err := daemon.DecideAndPublish(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(capture.input.Bundle, postID) || !strings.Contains(capture.input.Bundle, "correction linked") {
		t.Fatalf("bundle omitted matured prediction context: %s", capture.input.Bundle)
	}
}

func textNode(value string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Value: value} }
