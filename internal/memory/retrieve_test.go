package memory

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

func TestRetrieveContextStaysWithinBudgetAndKeepsProvenance(t *testing.T) {
	ctx := context.Background()
	runtime, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	base := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	for i := range 40 {
		kind := KindObservation
		if i < 3 {
			kind = KindOpenQuestion
		}
		if err := Write(ctx, runtime, Memory{
			ID: fmt.Sprintf("memory-%02d", i), Kind: kind,
			Summary:        strings.Repeat("界", 300),
			EvidenceLinks:  []string{fmt.Sprintf("https://example.org/source/%d", i)},
			RelatedPostIDs: []string{fmt.Sprintf("post-%02d", i)},
			UpdatedAt:      base.Add(time.Duration(i) * time.Minute), SourceHash: fmt.Sprintf("hash-%d", i),
		}); err != nil {
			t.Fatal(err)
		}
	}
	context, err := Retrieve(ctx, runtime, RetrievalOptions{
		Topics: []string{"technology"}, MaxItems: 8, MaxBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(context.Items) > 8 || len(context.JSON) > 4096 || !utf8.Valid(context.JSON) {
		t.Fatalf("context = %d items/%d bytes, valid=%t", len(context.Items), len(context.JSON), utf8.Valid(context.JSON))
	}
	if context.Items[0].Kind != KindOpenQuestion {
		t.Fatalf("first kind = %s, want unresolved question first", context.Items[0].Kind)
	}
	for _, item := range context.Items {
		if item.ID == "" || len(item.EvidenceLinks) == 0 || len(item.RelatedPostIDs) == 0 {
			t.Fatalf("context item lost provenance: %#v", item)
		}
	}
}

func TestRetrieveContextIsDeterministic(t *testing.T) {
	ctx := context.Background()
	runtime, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	at := time.Now().UTC()
	for _, id := range []string{"z-memory", "a-memory"} {
		if err := Write(ctx, runtime, Memory{
			ID: id, Kind: KindObservation, Summary: "summary", EvidenceLinks: []string{"https://example.org"},
			RelatedPostIDs: []string{"post-one"}, UpdatedAt: at, SourceHash: "hash",
		}); err != nil {
			t.Fatal(err)
		}
	}
	options := RetrievalOptions{MaxItems: 2, MaxBytes: 2048}
	first, err := Retrieve(ctx, runtime, options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Retrieve(ctx, runtime, options)
	if err != nil {
		t.Fatal(err)
	}
	if string(first.JSON) != string(second.JSON) || first.Items[0].ID != "a-memory" {
		t.Fatalf("context not deterministic: %#v vs %#v", first.Items, second.Items)
	}
}
