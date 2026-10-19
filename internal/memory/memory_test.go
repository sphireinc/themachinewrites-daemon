package memory

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

func TestWriteValidatesAndPreservesPriorMemoryOnFailure(t *testing.T) {
	ctx := context.Background()
	runtime, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	valid := Memory{
		ID: "topic-one", Kind: KindTopic, Summary: "A valid stored summary.",
		EvidenceLinks: []string{"https://example.org/source"}, RelatedPostIDs: []string{},
		UpdatedAt: at, SourceHash: "hash-one",
	}
	if err := Write(ctx, runtime, valid); err != nil {
		t.Fatal(err)
	}
	invalid := valid
	invalid.Summary = "\xff"
	if err := Write(ctx, runtime, invalid); err == nil {
		t.Fatal("invalid UTF-8 summary accepted")
	}
	memories, err := List(ctx, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) != 1 || memories[0].Summary != valid.Summary {
		t.Fatalf("memory rows = %#v", memories)
	}
}
