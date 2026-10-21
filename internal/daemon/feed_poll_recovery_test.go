package daemon

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/sources"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/supervisor"
)

type mixedFailureFetcher struct{ fetched []string }

func (f *mixedFailureFetcher) Fetch(_ context.Context, feedURL *url.URL, _ sources.Cursor) (sources.FetchResult, error) {
	f.fetched = append(f.fetched, feedURL.Host)
	switch feedURL.Host {
	case "fetch-failure.example":
		return sources.FetchResult{}, errors.New("temporary feed transport failure")
	case "parse-failure.example":
		return sources.FetchResult{Body: []byte(`{"error":"unsupported format"}`)}, nil
	case "healthy.example":
		return sources.FetchResult{Body: []byte(`<rss><channel><item><guid>healthy</guid><title>Healthy source item</title><link>https://healthy.example/story</link><pubDate>Sun, 18 Oct 2026 20:00:00 GMT</pubDate><description>Fixture excerpt.</description></item></channel></rss>`)}, nil
	default:
		return sources.FetchResult{}, errors.New("unexpected feed URL")
	}
}

func TestRuntimeOnceContinuesAfterFeedFetchAndParseFailures(t *testing.T) {
	ctx := context.Background()
	storeRuntime, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer storeRuntime.Close()

	makeSource := func(id, rawURL string) sources.Source {
		t.Helper()
		feedURL := &sources.URL{}
		if err := feedURL.UnmarshalYAML(textNode(rawURL)); err != nil {
			t.Fatal(err)
		}
		return sources.Source{ID: id, Name: id, Enabled: true, RequestInterval: time.Minute, URL: feedURL}
	}
	fetcher := &mixedFailureFetcher{}
	decision := &captureEngine{}
	runtime := Runtime{
		Sources: sources.Configuration{Sources: []sources.Source{
			makeSource("fetch-failure", "https://fetch-failure.example/feed.xml"),
			makeSource("parse-failure", "https://parse-failure.example/feed.xml"),
			makeSource("healthy", "https://healthy.example/feed.xml"),
		}},
		Fetcher: fetcher, Store: storeRuntime, Engine: decision, PublicRoot: t.TempDir(), DryRun: true,
		Now: func() time.Time { return time.Date(2026, 10, 18, 20, 1, 0, 0, time.UTC) },
	}
	cycle := supervisor.Cycle{Poll: runtime.Poll, Decide: runtime.DecideAndPublish, DryRun: true}
	if err := cycle.Once(ctx); err != nil {
		t.Fatalf("cycle failed despite recoverable feed errors: %v", err)
	}
	if len(fetcher.fetched) != 3 {
		t.Fatalf("fetch attempts = %v, want all three sources", fetcher.fetched)
	}
	if len(decision.input.Bundle) == 0 {
		t.Fatal("decision stage did not run with the healthy source observation")
	}
	if !strings.Contains(decision.input.Bundle, "Healthy source item") {
		t.Fatalf("decision bundle omitted the healthy source observation: %s", decision.input.Bundle)
	}
}

func TestRuntimePollKeepsCanceledCursorFailureFatal(t *testing.T) {
	storeRuntime, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer storeRuntime.Close()
	feedURL := &sources.URL{}
	if err := feedURL.UnmarshalYAML(textNode("https://healthy.example/feed.xml")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runtime := Runtime{
		Sources: sources.Configuration{Sources: []sources.Source{{ID: "healthy", Name: "Healthy", Enabled: true, RequestInterval: time.Minute, URL: feedURL}}},
		Fetcher: &mixedFailureFetcher{}, Store: storeRuntime,
	}
	if errs := runtime.Poll(ctx); len(errs) == 0 {
		t.Fatal("canceled source-cursor read was treated as a recoverable feed failure")
	}
}
