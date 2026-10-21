package sources

import (
	"context"
	"encoding/xml"
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

// PollError separates an external feed failure from failures persisting the
// source cursor or failure state. Feed failures can be logged and skipped;
// state failures mean polling progress could not be recorded safely.
type PollError struct {
	Feed  error
	State error
}

func (e *PollError) Error() string {
	return errors.Join(e.Feed, e.State).Error()
}

func (e *PollError) Unwrap() []error {
	var errs []error
	if e.Feed != nil {
		errs = append(errs, e.Feed)
	}
	if e.State != nil {
		errs = append(errs, e.State)
	}
	return errs
}

func (e *PollError) IsFeedFailureOnly() bool {
	return e != nil && e.Feed != nil && e.State == nil
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
		recordErr := cursors.RecordSourceFailure(ctx, source.ID, now, boundedPlainText(err.Error(), 500))
		return PollResult{}, &PollError{Feed: fmt.Errorf("fetch feed: %w", err), State: wrapCursorFailure(recordErr)}
	}
	if fetched.NotModified {
		return PollResult{NotModified: true}, nil
	}
	observations, err := parseFeed(fetched.Body, source, now)
	if err != nil {
		recordErr := cursors.RecordSourceFailure(ctx, source.ID, now, boundedPlainText(err.Error(), 500))
		return PollResult{}, &PollError{Feed: err, State: wrapCursorFailure(recordErr)}
	}
	if err := cursors.AdvanceSourceCursor(ctx, source.ID, fetched.ETag, fetched.LastModified, now); err != nil {
		return PollResult{}, err
	}
	return PollResult{Observations: observations}, nil
}

func parseFeed(content []byte, source Source, retrievedAt time.Time) ([]Observation, error) {
	trimmed := strings.TrimSpace(string(content))
	if !strings.HasPrefix(trimmed, "<?xml") && !strings.HasPrefix(trimmed, "<") {
		return nil, errors.New("unsupported feed format")
	}
	root, namespace := feedRootDetails(content)
	if root == "" {
		return nil, errors.New("feed root is missing")
	}
	switch root {
	case "rss", "RDF":
		return ParseRSS(content, source, retrievedAt)
	case "feed":
		if namespace != "http://www.w3.org/2005/Atom" {
			return nil, fmt.Errorf("unsupported Atom namespace %q", namespace)
		}
		return ParseAtom(content, source, retrievedAt)
	default:
		return nil, fmt.Errorf("unsupported feed format %q", root)
	}
}

func feedRoot(content []byte) (string, error) {
	name, _ := feedRootDetails(content)
	if name == "" {
		return "", errors.New("feed root is missing")
	}
	return name, nil
}

func feedRootDetails(content []byte) (string, string) {
	decoder := xml.NewDecoder(strings.NewReader(string(content)))
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", ""
		}
		if start, ok := token.(xml.StartElement); ok {
			return start.Name.Local, start.Name.Space
		}
	}
}

func wrapCursorFailure(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("record source failure: %w", err)
}
