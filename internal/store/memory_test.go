package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestMemoryMigrationRejectsDuplicateIDsAndInvalidNullFields(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	at := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := store.DB().ExecContext(ctx, `
        INSERT INTO memories(id, kind, summary, evidence_links, related_post_ids, updated_at, source_hash)
        VALUES ('memory-one', 'topic', 'summary', '[]', '[]', ?, 'hash')`, at); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `
        INSERT INTO memories(id, kind, summary, evidence_links, related_post_ids, updated_at, source_hash)
        VALUES ('memory-one', 'topic', 'summary', '[]', '[]', ?, 'hash')`, at); err == nil {
		t.Fatal("duplicate memory ID accepted")
	}
	if _, err := store.DB().ExecContext(ctx, `
        INSERT INTO memories(id, kind, summary, evidence_links, related_post_ids, updated_at, source_hash)
        VALUES ('memory-two', 'topic', NULL, '[]', '[]', ?, 'hash')`, at); err == nil {
		t.Fatal("NULL summary accepted")
	}
}
