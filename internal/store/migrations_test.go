package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
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

	err = store.migrate(ctx, []Migration{
		{Version: 1, Name: "one", SQL: "CREATE TABLE one (id INTEGER PRIMARY KEY);"},
		{Version: 2, Name: "bad", SQL: "CREATE TABLE two (id INTEGER PRIMARY KEY); INSERT INTO missing VALUES (1);"},
	})
	if err == nil || !strings.Contains(err.Error(), "migration 2 bad") {
		t.Fatalf("migrate() error = %v, want bounded migration failure", err)
	}
	if got := userVersion(t, store.DB()); got != 1 {
		t.Fatalf("user_version after rollback = %d, want 1", got)
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

func userVersion(t *testing.T, db interface{ QueryRow(string, ...any) *Row }) int {
	t.Helper()
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	return version
}

var _ = errors.Is
