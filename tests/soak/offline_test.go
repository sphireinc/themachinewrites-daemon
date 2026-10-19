package soak

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/gitpub"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/observations"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/sources"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/supervisor"
	"gopkg.in/yaml.v3"
)

type intermittentFetcher struct {
	now   time.Time
	calls int
}

func (f *intermittentFetcher) Fetch(context.Context, *url.URL, sources.Cursor) (sources.FetchResult, error) {
	f.calls++
	// Sustained but intermittent connectivity: every 25th fetch is offline.
	if f.calls%25 == 0 {
		return sources.FetchResult{}, errors.New("temporary network unavailable")
	}
	body := `<rss><channel><item><guid>soak-1</guid><title>One durable topic</title><link>https://example.org/soak-1</link><pubDate>Sun, 18 Oct 2026 20:00:00 GMT</pubDate><description>One durable observation.</description></item></channel></rss>`
	return sources.FetchResult{Body: []byte(body), ETag: "fixture-etag", LastModified: f.now.UTC().Format(time.RFC3339)}, nil
}

type cursorStore struct{ runtime *store.Store }

func (c cursorStore) SourceCursor(ctx context.Context, id string) (sources.CursorState, bool, error) {
	cur, found, err := c.runtime.SourceCursor(ctx, id)
	return sources.CursorState{ETag: cur.ETag, LastModified: cur.LastModified}, found, err
}
func (c cursorStore) AdvanceSourceCursor(ctx context.Context, id, etag, modified string, at time.Time) error {
	return c.runtime.AdvanceSourceCursor(ctx, id, etag, modified, at)
}
func (c cursorStore) RecordSourceFailure(ctx context.Context, id string, at time.Time, message string) error {
	return c.runtime.RecordSourceFailure(ctx, id, at, message)
}

type resilientPublisher struct {
	pushAttempts   int
	outageAttempts int
}

func (p *resilientPublisher) StageExact(context.Context, []string) error { return nil }
func (p *resilientPublisher) Commit(context.Context, string) (string, error) {
	return strings.Repeat("c", 40), nil
}
func (p *resilientPublisher) PushFastForward(_ context.Context, maxAttempts int) (string, error) {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		p.pushAttempts++
		if p.outageAttempts > 0 {
			p.outageAttempts--
			continue
		}
		return strings.Repeat("c", 40), nil
	}
	return "", errors.New("temporary network unavailable")
}
func (p *resilientPublisher) RecoverPost(context.Context, string) (string, error) {
	return "", gitpub.ErrPostNotPublished
}

func TestSevenDayIntermittentOfflineSoak(t *testing.T) {
	ctx := context.Background()
	database := filepath.Join(t.TempDir(), "runtime.sqlite")
	start := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	fetcher := &intermittentFetcher{now: start}
	backoff := supervisor.NewBackoff(time.Minute, time.Hour, func(int64) int64 { return 0 })
	publisher := &resilientPublisher{outageAttempts: 2}
	source := fixtureSource(t)
	previousRetry := time.Duration(0)
	maxRetry := time.Duration(0)
	sourceFailureCount := 0
	successfulPolls := 0
	persistent := openStore(t, database)

	for minute := 0; minute < 7*24*60; minute++ {
		now := start.Add(time.Duration(minute) * time.Minute)
		fetcher.now = now

		// Simulate supervisor/host restarts at fixed checkpoints.
		if minute > 0 && minute%360 == 0 {
			if err := persistent.Close(); err != nil {
				t.Fatal(err)
			}
			persistent = openStore(t, database)
		}

		result, err := sources.Poll(ctx, source, fetcher, cursorStore{persistent}, now)
		if err != nil {
			sourceFailureCount++
		} else {
			successfulPolls++
			for _, item := range result.Observations {
				if _, err := observations.Ingest(ctx, persistent, observations.Candidate{
					URL: item.URL, Headline: item.Headline, SourceID: item.SourceID, SourceName: item.SourceName,
					PublishedAt: item.PublishedAt, RetrievedAt: item.RetrievedAt,
				}); err != nil {
					t.Fatal(err)
				}
			}
		}

		// A quota/network failure resumes after bounded exponential backoff and
		// leaves the durable candidate queue intact.
		if minute%97 == 0 {
			if action := engine.Classify(context.DeadlineExceeded); action != engine.RetryAfterBackoff {
				t.Fatalf("quota classification = %s", action)
			}
			delay := backoff.Next()
			if delay <= 0 || delay > time.Hour || delay < previousRetry {
				t.Fatalf("unbounded backoff: previous=%s next=%s", previousRetry, delay)
			}
			previousRetry = delay
			if delay > maxRetry {
				maxRetry = delay
			}
		}

		pending, err := persistent.PendingCandidates(ctx, 100)
		if err != nil {
			t.Fatal(err)
		}
		if len(pending) > 1 {
			t.Fatalf("minute %d candidate queue grew beyond bounded unique item: %d", minute, len(pending))
		}
		_ = pending
	}

	if err := persistent.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openStore(t, database)
	defer reopened.Close()
	var candidates, cursors int
	if err := reopened.DB().QueryRowContext(ctx, "SELECT count(*) FROM observed_items").Scan(&candidates); err != nil {
		t.Fatal(err)
	}
	if err := reopened.DB().QueryRowContext(ctx, "SELECT count(*) FROM ingestion_cursors").Scan(&cursors); err != nil {
		t.Fatal(err)
	}
	if candidates != 1 {
		t.Fatalf("durable candidate count = %d, want one deduplicated item", candidates)
	}
	if cursors != 1 {
		t.Fatalf("durable cursor count = %d", cursors)
	}
	if sourceFailureCount == 0 || successfulPolls == 0 {
		t.Fatalf("soak did not exercise intermittent connectivity: failures=%d successes=%d", sourceFailureCount, successfulPolls)
	}
	if maxRetry > time.Hour || previousRetry == 0 {
		t.Fatalf("retry rate unbounded or absent: max=%s", maxRetry)
	}

	// A restart-safe publication attempt has a hard retry ceiling and recovers
	// after transient Git unavailability without creating a second post ID.
	const postID = "01M589K0G00000000000000000"
	if err := reopened.SavePendingPublication(ctx, postID); err != nil {
		t.Fatal(err)
	}
	result, err := gitpub.Publish(ctx, publisher, reopened, gitpub.PublishRequest{
		PostID: postID, Paths: []string{"posts/fixture.md"}, MaxPushAttempts: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CommitSHA != strings.Repeat("c", 40) || publisher.pushAttempts != 3 {
		t.Fatalf("publication retry result=%#v attempts=%d", result, publisher.pushAttempts)
	}
	exhausted := &resilientPublisher{outageAttempts: 100}
	if _, err := gitpub.Publish(ctx, exhausted, reopened, gitpub.PublishRequest{
		PostID: "01M589K0G00000000000000001", Paths: []string{"posts/exhausted.md"}, MaxPushAttempts: 3,
	}); err == nil {
		t.Fatal("unrecovered outage did not reach a bounded failure")
	}
	if exhausted.pushAttempts != 3 {
		t.Fatalf("exhausted outage attempted %d pushes, want bounded 3", exhausted.pushAttempts)
	}
	var published int
	if err := reopened.DB().QueryRowContext(ctx, "SELECT count(*) FROM publication_outbox WHERE state = 'published'").Scan(&published); err != nil {
		t.Fatal(err)
	}
	if published != 1 {
		t.Fatalf("published outbox count = %d", published)
	}
	t.Logf("7-day soak: polls=%d failures=%d successes=%d retry_max=%s durable_candidates=%d", fetcher.calls, sourceFailureCount, successfulPolls, maxRetry, candidates)
}

func fixtureSource(t *testing.T) sources.Source {
	t.Helper()
	var source sources.Source
	if err := yaml.Unmarshal([]byte("id: fixture\nname: Fixture\nurl: https://example.org/feed.xml\nenabled: true\nrequest_interval: 1m\n"), &source); err != nil {
		t.Fatal(err)
	}
	return source
}

func openStore(t *testing.T, path string) *store.Store {
	t.Helper()
	runtime, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}
