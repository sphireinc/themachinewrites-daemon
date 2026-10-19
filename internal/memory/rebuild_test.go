package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRebuildFindsCanonicalPostIDsAfterDatabaseLoss(t *testing.T) {
	ctx := context.Background()
	public := t.TempDir()
	ids := []string{"01M589K0G00000000000000000", "01M589K0G00000000000000001"}
	for index, id := range ids {
		directory := filepath.Join(public, "posts", "2026", "10", "18")
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		content := []byte("---\nid: " + id + "\n---\nbody\n")
		if err := os.WriteFile(filepath.Join(directory, id+".md"), content, 0o600); err != nil {
			t.Fatal(err)
		}
		_ = index
	}
	rebuilt, err := RebuildFromPublic(ctx, public)
	if err != nil {
		t.Fatal(err)
	}
	if len(rebuilt.PostIDs) != 2 || rebuilt.PostIDs[0] != ids[0] || rebuilt.PostIDs[1] != ids[1] {
		t.Fatalf("rebuilt IDs = %#v", rebuilt.PostIDs)
	}
}

func TestRebuildIgnoresInvalidULIDAndMisdatedPath(t *testing.T) {
	ctx := context.Background()
	public := t.TempDir()
	badDirectory := filepath.Join(public, "posts", "1999", "01", "01")
	if err := os.MkdirAll(badDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"not-a-valid-ulid-value-0000.md",
		"01M589K0G00000000000000000.md",
	} {
		if err := os.WriteFile(filepath.Join(badDirectory, name), []byte("---\n---\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rebuilt, err := RebuildFromPublic(ctx, public)
	if err != nil {
		t.Fatal(err)
	}
	if len(rebuilt.PostIDs) != 0 {
		t.Fatalf("rebuild accepted invalid paths: %#v", rebuilt.PostIDs)
	}
}
