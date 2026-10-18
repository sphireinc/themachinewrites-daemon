package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWALRestartPreservesQueueAndPassesIntegrityCheck(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "runtime.sqlite")
	now := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)

	first, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.DB().ExecContext(ctx, `
        INSERT INTO jobs(job_id, state, created_at, updated_at)
        VALUES ('job-durable', 'pending', ?, ?)`,
		now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	assertIntegrityOK(t, second.DB())
	claim, err := second.ClaimJob(ctx, "restart-worker", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !claim.Found || claim.JobID != "job-durable" {
		t.Fatalf("claim after restart = %#v, want durable job", claim)
	}
}

func TestInterruptedTransactionRollsBackAndDatabaseRemainsUsable(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	now := "2026-10-18T20:00:00Z"
	if _, err := tx.ExecContext(ctx, `
        INSERT INTO jobs(job_id, state, created_at, updated_at)
        VALUES ('job-interrupted', 'pending', ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.DB().QueryRowContext(ctx,
		"SELECT count(*) FROM jobs WHERE job_id = 'job-interrupted'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rolled-back job count = %d, want 0", count)
	}
	assertIntegrityOK(t, store.DB())
}

func TestMigrationFailureLeavesDatabaseAtPreviousVersion(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "runtime.sqlite")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	next := append(append([]Migration(nil), migrations...), Migration{
		Version: CurrentSchemaVersion + 1,
		Name:    "injected_failure",
		SQL:     "CREATE TABLE injected_failure (id INTEGER); INSERT INTO absent_table VALUES (1);",
	})
	err = store.migrate(ctx, next)
	if err == nil || !strings.Contains(err.Error(), "injected_failure") {
		t.Fatalf("migrate() error = %v, want injected failure", err)
	}
	if got := userVersion(t, store.DB()); got != CurrentSchemaVersion {
		t.Fatalf("user_version = %d, want %d", got, CurrentSchemaVersion)
	}
	var count int
	if err := store.DB().QueryRowContext(ctx,
		"SELECT count(*) FROM sqlite_master WHERE type='table' AND name='injected_failure'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed migration table remained")
	}
}

func TestLockedDatabaseFailsWithBoundedTimeout(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "runtime.sqlite")
	first, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()

	tx, err := first.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "UPDATE jobs SET updated_at = updated_at WHERE 0"); err != nil {
		t.Fatal(err)
	}

	lockCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	second, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if _, err := second.ExecContext(lockCtx, "PRAGMA busy_timeout = 25"); err != nil {
		t.Fatal(err)
	}
	_, err = second.ExecContext(lockCtx, `
        INSERT INTO jobs(job_id, state, created_at, updated_at)
        VALUES ('job-locked', 'pending', '2026-10-18T20:00:00Z', '2026-10-18T20:00:00Z')`)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "locked") {
		t.Fatalf("locked insert error = %v, want bounded locked-database failure", err)
	}
}

func assertIntegrityOK(t *testing.T, db *sql.DB) {
	t.Helper()
	var result string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		t.Fatal(err)
	}
	if result != "ok" {
		t.Fatalf("PRAGMA integrity_check = %q, want ok", result)
	}
}
