package memory

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

func TestSecondDaemonPassRecallsPriorTopicAndPostAfterRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "runtime.sqlite")
	firstRuntime, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	if err := Write(ctx, firstRuntime, Memory{
		ID: "technology-prior", Kind: KindObservation,
		Summary: "Technology prior conclusion.", EvidenceLinks: []string{"https://example.org/source"},
		RelatedPostIDs: []string{"01M589K0G00000000000000000"}, UpdatedAt: at, SourceHash: "hash",
	}); err != nil {
		t.Fatal(err)
	}
	if err := firstRuntime.Close(); err != nil {
		t.Fatal(err)
	}

	secondRuntime, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer secondRuntime.Close()
	context, err := Retrieve(ctx, secondRuntime, RetrievalOptions{Topics: []string{"technology"}, MaxItems: 10, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(context.JSON), "technology") ||
		!strings.Contains(string(context.JSON), "01M589K0G00000000000000000") {
		t.Fatalf("second prompt context lost prior evidence: %s", context.JSON)
	}
}
