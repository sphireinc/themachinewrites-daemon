package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestOpenCreatesV1SchemaAndSecondOpenDoesNotReapply(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "runtime.sqlite")

	first, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	if got := userVersion(t, first.DB()); got != CurrentSchemaVersion {
		t.Fatalf("user_version = %d, want %d", got, CurrentSchemaVersion)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	defer second.Close()
	if got := userVersion(t, second.DB()); got != CurrentSchemaVersion {
		t.Fatalf("second user_version = %d, want %d", got, CurrentSchemaVersion)
	}
}

func TestMigrateRollsBackFailedMigration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "runtime.sqlite")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.DB().ExecContext(ctx, "PRAGMA user_version = 0"); err != nil {
		t.Fatal(err)
	}

	err = store.migrate(ctx, []Migration{
		{Version: 1, Name: "one", SQL: "CREATE TABLE one (id INTEGER PRIMARY KEY);"},
		{Version: 2, Name: "bad", SQL: "CREATE TABLE two (id INTEGER PRIMARY KEY); INSERT INTO missing VALUES (1);"},
	})
	if err == nil || !strings.Contains(err.Error(), "migration 2 bad") {
		t.Fatalf("migrate() error = %v, want bounded migration failure", err)
	}
	if got := userVersion(t, store.DB()); got != 0 {
		t.Fatalf("user_version after rollback = %d, want 0", got)
	}
	var oneExists int
	if err := store.DB().QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='one'").Scan(&oneExists); err != nil {
		t.Fatal(err)
	}
	if oneExists != 0 {
		t.Fatal("migration one remained committed after migration two failed")
	}
	var exists int
	if err := store.DB().QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='two'").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists != 0 {
		t.Fatal("partially applied migration remained after rollback")
	}
}

func TestMigrateRejectsNonSequentialVersionsAndNewerDatabase(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.migrate(ctx, []Migration{{Version: 2, Name: "gap", SQL: "SELECT 1;"}}); err == nil {
		t.Fatal("migrate accepted a version gap")
	}
	if _, err := store.DB().ExecContext(ctx, "PRAGMA user_version = 999"); err != nil {
		t.Fatal(err)
	}
	if err := store.migrate(ctx, migrations); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("migrate() error = %v, want newer-schema rejection", err)
	}
}

func TestV1SchemaIncludesUniqueIngestionCursors(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO ingestion_cursors(source_id, etag, last_modified, last_checked_at, error_count) VALUES (?, NULL, NULL, ?, 0)",
		"source-one", "2026-10-18T20:00:00Z"); err != nil {
		t.Fatalf("insert cursor: %v", err)
	}
	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO ingestion_cursors(source_id, etag, last_modified, last_checked_at, error_count) VALUES (?, NULL, NULL, ?, 0)",
		"source-one", "2026-10-18T20:01:00Z"); err == nil {
		t.Fatal("duplicate source_id was accepted")
	}
}

func TestObservedItemsDeduplicateByFingerprint(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	inserted, err := store.ObserveItem(ctx, ObservedItem{
		URLCanonicalHash: "url-hash",
		SourceID:         "source-one",
		DiscoveredAt:     "2026-10-18T20:00:00Z",
		Fingerprint:      "fingerprint-one",
		LastSeenAt:       "2026-10-18T20:00:00Z",
	})
	if err != nil || !inserted {
		t.Fatalf("first ObserveItem() = %t, %v; want inserted", inserted, err)
	}
	inserted, err = store.ObserveItem(ctx, ObservedItem{
		URLCanonicalHash: "different-url",
		SourceID:         "source-two",
		DiscoveredAt:     "2026-10-18T20:01:00Z",
		Fingerprint:      "fingerprint-one",
		LastSeenAt:       "2026-10-18T20:01:00Z",
	})
	if err != nil || inserted {
		t.Fatalf("duplicate ObserveItem() = %t, %v; want deduplicated", inserted, err)
	}
	var lastSeen string
	if err := store.DB().QueryRowContext(ctx,
		"SELECT last_seen_at FROM observed_items WHERE fingerprint = ?", "fingerprint-one").Scan(&lastSeen); err != nil {
		t.Fatal(err)
	}
	if lastSeen != "2026-10-18T20:01:00Z" {
		t.Fatalf("last_seen_at = %q, want refreshed timestamp", lastSeen)
	}
}

func TestObservedItemRejectsInvalidTimestampsWithoutInsert(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_, err = store.ObserveItem(ctx, ObservedItem{
		URLCanonicalHash: "url-hash",
		SourceID:         "source-one",
		DiscoveredAt:     "not-a-time",
		Fingerprint:      "fingerprint-invalid",
		LastSeenAt:       "2026-10-18T20:00:00Z",
	})
	if err == nil || !strings.Contains(err.Error(), "RFC3339") {
		t.Fatalf("ObserveItem() error = %v, want timestamp rejection", err)
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, "SELECT count(*) FROM observed_items").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("observed_items count = %d, want 0", count)
	}
}

func TestConcurrentObservedItemsDeduplicateWithoutFatalError(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	item := ObservedItem{
		URLCanonicalHash: "url-hash",
		SourceID:         "source-one",
		DiscoveredAt:     "2026-10-18T20:00:00Z",
		Fingerprint:      "fingerprint-concurrent",
		LastSeenAt:       "2026-10-18T20:00:00Z",
	}

	start := make(chan struct{})
	results := make(chan bool, 2)
	errors := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			inserted, err := store.ObserveItem(ctx, item)
			results <- inserted
			errors <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errors)

	insertedCount := 0
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent ObserveItem() error = %v", err)
		}
	}
	for inserted := range results {
		if inserted {
			insertedCount++
		}
	}
	if insertedCount != 1 {
		t.Fatalf("inserted count = %d, want exactly 1", insertedCount)
	}
}

func TestObservedItemLastSeenDoesNotMoveBackward(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	item := ObservedItem{
		URLCanonicalHash: "url-hash",
		SourceID:         "source-one",
		DiscoveredAt:     "2026-10-18T20:00:00Z",
		Fingerprint:      "fingerprint-monotonic",
		LastSeenAt:       "2026-10-18T20:02:00Z",
	}
	if inserted, err := store.ObserveItem(ctx, item); err != nil || !inserted {
		t.Fatalf("first ObserveItem() = %t, %v", inserted, err)
	}
	item.LastSeenAt = "2026-10-18T20:01:00Z"
	if inserted, err := store.ObserveItem(ctx, item); err != nil || inserted {
		t.Fatalf("repeat ObserveItem() = %t, %v", inserted, err)
	}
	var lastSeen string
	if err := store.DB().QueryRowContext(ctx,
		"SELECT last_seen_at FROM observed_items WHERE fingerprint = ?", item.Fingerprint).Scan(&lastSeen); err != nil {
		t.Fatal(err)
	}
	if lastSeen != "2026-10-18T20:02:00Z" {
		t.Fatalf("last_seen_at = %q, want newer timestamp preserved", lastSeen)
	}
}

func userVersion(t *testing.T, db interface{ QueryRow(string, ...any) *Row }) int {
	t.Helper()
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	return version
}

var _ = errors.Is
