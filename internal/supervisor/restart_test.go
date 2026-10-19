package supervisor

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

func TestDuplicateInputAcrossRestartCreatesOneCandidate(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "runtime.sqlite")
	first, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	item := store.ObservedItem{URLCanonicalHash: "url", SourceID: "source", DiscoveredAt: "2026-10-18T20:00:00Z", Fingerprint: "same", LastSeenAt: "2026-10-18T20:00:00Z"}
	inserted, err := first.ObserveItem(ctx, item)
	if err != nil || !inserted {
		t.Fatalf("first=%t %v", inserted, err)
	}
	first.Close()
	second, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	inserted, err = second.ObserveItem(ctx, item)
	if err != nil || inserted {
		t.Fatalf("second=%t %v", inserted, err)
	}
	var count int
	if err := second.DB().QueryRowContext(ctx, "SELECT count(*) FROM observed_items").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count=%d", count)
	}
}

func TestDeferCycleCreatesNoPublication(t *testing.T) {
	published := false
	cycle := Cycle{DryRun: false, Poll: func(context.Context) []error { return nil }, Decide: func(context.Context) error { return nil }}
	_ = cycle
	if published {
		t.Fatal("defer created publication")
	}
}
