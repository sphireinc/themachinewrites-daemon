package memory

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

func TestMemoryReconciliationRequiresConfirmedCommit(t *testing.T) {
	ctx := context.Background()
	runtime, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	if err := Write(ctx, runtime, Memory{
		ID: "topic", Kind: KindTopic, Summary: "Technology topic", EvidenceLinks: []string{"https://example.org"},
		RelatedPostIDs: []string{"post-one"}, UpdatedAt: at, SourceHash: "hash",
	}); err != nil {
		t.Fatal(err)
	}
	if err := Reconcile(ctx, runtime, "post-one", ""); err == nil {
		t.Fatal("empty remote commit accepted")
	}
	before, err := List(ctx, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if before[0].CommitSHA != "" {
		t.Fatalf("memory marked published before confirmation: %#v", before[0])
	}
	if err := Reconcile(ctx, runtime, "post-one", "sha-confirmed"); err != nil {
		t.Fatal(err)
	}
	after, err := List(ctx, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if after[0].CommitSHA != "sha-confirmed" {
		t.Fatalf("commit SHA = %q, want confirmed", after[0].CommitSHA)
	}
}
