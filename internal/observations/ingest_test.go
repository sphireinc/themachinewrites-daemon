package observations

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

func TestTwoIdenticalPollsProduceOneCandidate(t *testing.T) {
	ctx := context.Background()
	runtime, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	item := Candidate{
		URL:         "https://example.org/article?id=7&utm_source=feed#top",
		Headline:    "A source headline",
		SourceID:    "source-one",
		SourceName:  "Source One",
		Category:    "technology",
		PublishedAt: time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC),
		Excerpt:     "A bounded excerpt.",
		RetrievedAt: time.Date(2026, 10, 18, 20, 1, 0, 0, time.UTC),
	}
	first, err := Ingest(ctx, runtime, item)
	if err != nil || !first {
		t.Fatalf("first Ingest() = %t, %v", first, err)
	}
	second, err := Ingest(ctx, runtime, item)
	if err != nil || second {
		t.Fatalf("second Ingest() = %t, %v", second, err)
	}
	var count int
	if err := runtime.DB().QueryRowContext(ctx, "SELECT count(*) FROM observed_items").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("candidate count = %d, want 1", count)
	}
}
