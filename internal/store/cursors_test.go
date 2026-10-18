package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSourceCursorRoundTripAndSuccessfulAdvance(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	cursor, found, err := store.SourceCursor(ctx, "source-one")
	if err != nil || found {
		t.Fatalf("initial SourceCursor() = %#v, %t, %v", cursor, found, err)
	}
	if err := store.AdvanceSourceCursor(ctx, "source-one", `"etag"`, "Sun, 18 Oct 2026 20:00:00 GMT", at); err != nil {
		t.Fatal(err)
	}
	cursor, found, err = store.SourceCursor(ctx, "source-one")
	if err != nil || !found || cursor.ETag != `"etag"` || cursor.LastModified != "Sun, 18 Oct 2026 20:00:00 GMT" ||
		!cursor.LastCheckedAt.Equal(at) || cursor.ErrorCount != 0 {
		t.Fatalf("SourceCursor() = %#v, %t, %v", cursor, found, err)
	}
}

func TestSourceCursorFailureIncrementDoesNotChangeValidators(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	if err := store.AdvanceSourceCursor(ctx, "source-one", "etag-old", "modified-old", at); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordSourceFailure(ctx, "source-one", at.Add(time.Minute), "temporary failure"); err != nil {
		t.Fatal(err)
	}
	cursor, found, err := store.SourceCursor(ctx, "source-one")
	if err != nil || !found {
		t.Fatal(err)
	}
	if cursor.ETag != "etag-old" || cursor.LastModified != "modified-old" || cursor.ErrorCount != 1 ||
		!cursor.LastCheckedAt.Equal(at.Add(time.Minute)) || cursor.LastError != "temporary failure" {
		t.Fatalf("cursor after failure = %#v", cursor)
	}
}

func TestSourceCursorRejectsInvalidSource(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if err := store.AdvanceSourceCursor(ctx, "  ", "etag", "", time.Now()); err == nil {
		t.Fatal("empty source ID accepted")
	}
	var found bool
	_ = found
	if _, _, err := store.SourceCursor(ctx, "missing"); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("missing SourceCursor() error = %v", err)
	}
}
