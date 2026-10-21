package daemon

import (
	"bytes"
	"context"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/logging"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/memory"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/observations"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/publication"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/sources"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/supervisor"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/telemetry"
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

type publishEngine struct{}

func (publishEngine) Run(context.Context, engine.Input) (engine.Proposal, error) {
	return engine.Proposal{
		Action: engine.ActionPublish, Reason: "Fixture publication.",
		Title: "A fixture publication", Type: "observation", Body: "A complete fixture observation.",
		Sources: []engine.Source{{Title: "Fixture", URL: "https://example.org/source", RetrievedAt: "2026-10-18T20:00:00Z"}},
		Topics:  []string{"technology"}, ConfidenceRationale: "One fixture source.",
	}, nil
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

func TestRuntimeLogsSafeOnceHotPathMilestones(t *testing.T) {
	ctx := context.Background()
	storeRuntime, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer storeRuntime.Close()
	sourceURL := &sources.URL{}
	if err := sourceURL.UnmarshalYAML(textNode("https://example.org/private-feed.xml")); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	runtime := Runtime{
		Sources: sources.Configuration{Sources: []sources.Source{{ID: "fixture", Name: "Fixture", Enabled: true, RequestInterval: time.Minute, URL: sourceURL}}},
		Fetcher: fakeFetcher{}, Store: storeRuntime, Engine: deferEngine{}, PublicRoot: t.TempDir(), DryRun: true,
		Logger: logging.New(&logs),
	}
	if errs := runtime.Poll(ctx); len(errs) != 0 {
		t.Fatalf("poll errors: %v", errs)
	}
	if err := runtime.DecideAndPublish(ctx); err != nil {
		t.Fatal(err)
	}
	output := logs.String()
	for _, event := range []string{"source.poll.started", "source.poll.completed", "decision.candidates", "codex.started", "codex.completed", "decision.deferred"} {
		if !strings.Contains(output, event) {
			t.Errorf("logs missing %q: %s", event, output)
		}
	}
	for _, sensitive := range []string{"Fixture headline", "Fixture source excerpt", "private-feed.xml", "No contribution."} {
		if strings.Contains(output, sensitive) {
			t.Errorf("logs contain content %q: %s", sensitive, output)
		}
	}
}

func TestRuntimeRecordsDeferHealthAcrossSuccessfulFetches(t *testing.T) {
	ctx := context.Background()
	runtime, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	sourceURL := &sources.URL{}
	if err := sourceURL.UnmarshalYAML(textNode("https://example.org/feed.xml")); err != nil {
		t.Fatal(err)
	}
	healthPath := filepath.Join(t.TempDir(), "health.json")
	at := time.Date(2026, 10, 18, 20, 1, 0, 0, time.UTC)
	now := at
	daemonRuntime := Runtime{
		Sources: sources.Configuration{Sources: []sources.Source{{ID: "fixture", Name: "Fixture", Enabled: true, RequestInterval: time.Minute, URL: sourceURL}}},
		Fetcher: fakeFetcher{}, Store: runtime, Engine: deferEngine{}, PublicRoot: t.TempDir(),
		HealthPath: healthPath, DryRun: false, Now: func() time.Time { return now },
	}
	for cycle := 0; cycle < 3; cycle++ {
		if errs := daemonRuntime.Poll(ctx); len(errs) > 0 {
			t.Fatalf("cycle %d poll errors: %v", cycle, errs)
		}
		if err := daemonRuntime.DecideAndPublish(ctx); err != nil {
			t.Fatalf("cycle %d decision: %v", cycle, err)
		}
		now = now.Add(time.Minute)
	}
	health, err := telemetry.ReadHealth(healthPath)
	if err != nil {
		t.Fatal(err)
	}
	if health.LastPoll == nil || health.LastModelCall == nil || health.LastConfirmedPush != nil {
		t.Fatalf("defer health = %#v", health)
	}
	if health.PausedReason != "insufficient_evidence" || health.LastErrorKind != "" {
		t.Fatalf("defer health labels = %#v", health)
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
	}, publication.RenderInput{ID: postID, PublishedAt: postedAt, ModelIdentifier: "fixture", PromptVersion: "p1", IndependentSources: 1})
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
	if !strings.Contains(capture.input.Bundle, postID) || !strings.Contains(capture.input.Bundle, "active_dispute") || !strings.Contains(capture.input.Bundle, "correction linked") {
		t.Fatalf("bundle omitted matured prediction context: %s", capture.input.Bundle)
	}
}

func TestBudgetExhaustionLeavesCandidatePendingForNextDay(t *testing.T) {
	ctx := context.Background()
	runtime, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	if _, err := observations.Ingest(ctx, runtime, observations.Candidate{
		URL: "https://example.org/one", Headline: "Fixture headline", SourceID: "fixture", SourceName: "Fixture",
		PublishedAt: at, RetrievedAt: at,
	}); err != nil {
		t.Fatal(err)
	}
	budget := supervisor.NewBudget(at, supervisor.BudgetLimits{Invocations: 0, InputTokens: 1, OutputTokens: 1, MaxRetries: 1})
	daemon := Runtime{Store: runtime, Engine: deferEngine{}, PublicRoot: t.TempDir(), Now: func() time.Time { return at }, Budget: budget}
	if err := daemon.DecideAndPublish(ctx); err == nil {
		t.Fatal("budget-exhausted cycle unexpectedly succeeded")
	}
	candidates, err := runtime.PendingCandidates(ctx, 10)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("budget failure stranded candidate: %#v, %v", candidates, err)
	}
}

func TestDryRunPublicationKeepsCandidatePending(t *testing.T) {
	ctx := context.Background()
	runtime, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	if _, err := observations.Ingest(ctx, runtime, observations.Candidate{
		URL: "https://example.org/one", Headline: "Fixture headline", SourceID: "fixture", SourceName: "Fixture",
		PublishedAt: at, RetrievedAt: at,
	}); err != nil {
		t.Fatal(err)
	}
	publicRoot := t.TempDir()
	daemon := Runtime{Store: runtime, Engine: publishEngine{}, PublicRoot: publicRoot, DryRun: true, Now: func() time.Time { return at }}
	if err := daemon.DecideAndPublish(ctx); err != nil {
		t.Fatal(err)
	}
	candidates, err := runtime.PendingCandidates(ctx, 10)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("dry run did not retain candidate: %#v, %v", candidates, err)
	}
	if _, err := os.Stat(filepath.Join(publicRoot, "posts")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote publication files: %v", err)
	}
}

func TestInterruptedCandidateReusesReservedPostID(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "runtime.sqlite")
	runtime, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	if _, err := observations.Ingest(ctx, runtime, observations.Candidate{
		URL: "https://example.org/one", Headline: "Fixture headline", SourceID: "fixture", SourceName: "Fixture",
		PublishedAt: at, RetrievedAt: at,
	}); err != nil {
		t.Fatal(err)
	}
	candidates, err := runtime.PendingCandidates(ctx, 10)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates = %#v, %v", candidates, err)
	}
	reservedAt := at.Truncate(time.Millisecond)
	reservedID, err := publication.GenerateID(candidates[0].Fingerprint, reservedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.ReserveCandidates(ctx, []store.CandidateReservation{{Fingerprint: candidates[0].Fingerprint, PostID: reservedID, At: reservedAt}}); err != nil {
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
	if _, err := runtime.RequeueInterruptedCandidates(ctx, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	candidates, err = runtime.PendingCandidates(ctx, 10)
	if err != nil || len(candidates) != 1 || candidates[0].DecisionPostID != reservedID || candidates[0].PublicationAt == "" {
		t.Fatalf("reservation lost across restart: %#v, %v", candidates, err)
	}
}

func textNode(value string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Value: value} }
