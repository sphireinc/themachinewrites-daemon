package sources

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"
)

type fakeFetcher struct {
	result FetchResult
	err    error
	cursor Cursor
}

func (f *fakeFetcher) Fetch(_ context.Context, _ *url.URL, cursor Cursor) (FetchResult, error) {
	f.cursor = cursor
	return f.result, f.err
}

type fakeCursorStore struct {
	cursor      CursorState
	found       bool
	advanced    bool
	failure     error
	failureAt   time.Time
	failureText string
}

func (s *fakeCursorStore) SourceCursor(context.Context, string) (CursorState, bool, error) {
	return s.cursor, s.found, nil
}

func (s *fakeCursorStore) AdvanceSourceCursor(_ context.Context, _ string, _, _ string, _ time.Time) error {
	s.advanced = true
	return nil
}

func (s *fakeCursorStore) RecordSourceFailure(_ context.Context, _ string, at time.Time, message string) error {
	s.failure = errors.New("recorded")
	s.failureAt = at
	s.failureText = message
	return nil
}

func TestPollAdvancesCursorOnlyAfterSuccessfulParse(t *testing.T) {
	rss := `<rss><channel><item><guid>a</guid><title>One</title><link>https://example.org/a</link><pubDate>Sun, 18 Oct 2026 20:00:00 GMT</pubDate><description>One</description></item><item><guid>b</guid><title>Two</title><link>https://example.org/b</link><pubDate>Sun, 18 Oct 2026 21:00:00 GMT</pubDate><description>Two</description></item></channel></rss>`
	fetcher := &fakeFetcher{result: FetchResult{Body: []byte(rss), ETag: "etag-new", LastModified: "modified-new"}}
	cursors := &fakeCursorStore{
		cursor: CursorState{ETag: "etag-old", LastModified: "modified-old"},
		found:  true,
	}
	now := time.Date(2026, 10, 18, 22, 0, 0, 0, time.UTC)
	result, err := Poll(context.Background(), testSource(t, "rss", "RSS"), fetcher, cursors, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Observations) != 2 || !cursors.advanced || result.NotModified {
		t.Fatalf("Poll() result = %#v, advanced = %t", result, cursors.advanced)
	}
	if fetcher.cursor.ETag != "etag-old" || fetcher.cursor.LastModified != "modified-old" {
		t.Fatalf("fetcher cursor = %#v", fetcher.cursor)
	}
}

func TestPollDoesNotAdvanceCursorOnFailureOrMalformedFeed(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name    string
		fetcher *fakeFetcher
		body    string
	}{
		{name: "HTTP failure", fetcher: &fakeFetcher{err: errors.New("HTTP 500")}},
		{name: "malformed feed", fetcher: &fakeFetcher{result: FetchResult{Body: []byte(`<rss><channel>`)}}},
		{name: "timeout", fetcher: &fakeFetcher{err: context.DeadlineExceeded}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cursors := &fakeCursorStore{cursor: CursorState{ETag: "etag-old"}, found: true}
			_, err := Poll(context.Background(), testSource(t, "source", "Source"), tt.fetcher, cursors, now)
			if err == nil {
				t.Fatal("Poll() accepted failure")
			}
			if cursors.advanced {
				t.Fatal("cursor advanced after failure")
			}
			if cursors.failure == nil {
				t.Fatal("source failure was not recorded")
			}
		})
	}
}

func TestPollNotModifiedPreservesCursor(t *testing.T) {
	cursors := &fakeCursorStore{cursor: CursorState{ETag: "etag-old"}, found: true}
	result, err := Poll(context.Background(), testSource(t, "source", "Source"),
		&fakeFetcher{result: FetchResult{NotModified: true}}, cursors, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if !result.NotModified || cursors.advanced {
		t.Fatalf("not-modified result = %#v, advanced = %t", result, cursors.advanced)
	}
}

func testSource(t *testing.T, id, name string) Source {
	t.Helper()
	return Source{
		ID:   id,
		Name: name,
		URL:  &URL{value: mustURL(t, "https://example.org/feed.xml")},
	}
}
