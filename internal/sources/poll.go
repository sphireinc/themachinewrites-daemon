package sources

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type FeedFetcher interface {
	Fetch(context.Context, *url.URL, Cursor) (FetchResult, error)
}

type CursorState struct {
	ETag         string
	LastModified string
}

type CursorStore interface {
	SourceCursor(context.Context, string) (CursorState, bool, error)
	AdvanceSourceCursor(context.Context, string, string, string, time.Time) error
	RecordSourceFailure(context.Context, string, time.Time, string) error
}

type PollResult struct {
	Observations []Observation
	NotModified  bool
}

func Poll(ctx context.Context, source Source, fetcher FeedFetcher, cursors CursorStore, now time.Time) (PollResult, error) {
	if fetcher == nil || cursors == nil || source.URL == nil || source.URL.Parsed() == nil {
		return PollResult{}, errors.New("source poller is not configured")
	}
	cursor, _, err := cursors.SourceCursor(ctx, source.ID)
	if err != nil {
		return PollResult{}, err
	}
	fetched, err := fetcher.Fetch(ctx, source.URL.Parsed(), Cursor{ETag: cursor.ETag, LastModified: cursor.LastModified})
	if err != nil {
		_ = cursors.RecordSourceFailure(ctx, source.ID, now, boundedPlainText(err.Error(), 500))
		return PollResult{}, err
	}
	if fetched.NotModified {
		return PollResult{NotModified: true}, nil
	}
	observations, err := parseFeed(fetched.Body, source, now)
	if err != nil {
		_ = cursors.RecordSourceFailure(ctx, source.ID, now, boundedPlainText(err.Error(), 500))
		return PollResult{}, err
	}
	if err := cursors.AdvanceSourceCursor(ctx, source.ID, fetched.ETag, fetched.LastModified, now); err != nil {
		return PollResult{}, err
	}
	return PollResult{Observations: observations}, nil
}

func parseFeed(content []byte, source Source, retrievedAt time.Time) ([]Observation, error) {
	trimmed := strings.TrimSpace(string(content))
	switch {
	case strings.Contains(trimmed, "<feed") && strings.Contains(trimmed, "http://www.w3.org/2005/Atom"):
		return ParseAtom(content, source, retrievedAt)
	case strings.Contains(trimmed, "<rss"):
		return ParseRSS(content, source, retrievedAt)
	default:
		return nil, fmt.Errorf("unsupported feed format")
	}
}
