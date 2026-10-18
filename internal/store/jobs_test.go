package store

import (
	"context"
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestJobsAndOutboxHaveRequiredUniquenessAndStateConstraints(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	now := "2026-10-18T20:00:00Z"
	if _, err := store.DB().ExecContext(ctx, `
        INSERT INTO jobs(job_id, state, created_at, updated_at)
        VALUES ('job-one', 'pending', ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `
        INSERT INTO jobs(job_id, state, created_at, updated_at)
        VALUES ('job-one', 'pending', ?, ?)`, now, now); err == nil {
		t.Fatal("duplicate job_id was accepted")
	}
	if _, err := store.DB().ExecContext(ctx, `
        INSERT INTO jobs(job_id, state, created_at, updated_at)
        VALUES ('job-invalid', 'unknown', ?, ?)`, now, now); err == nil {
		t.Fatal("invalid job state was accepted")
	}
	if _, err := store.DB().ExecContext(ctx, `
        INSERT INTO jobs(job_id, state, created_at, updated_at)
        VALUES ('job-missing-lease', 'leased', ?, ?)`, now, now); err == nil {
		t.Fatal("leased job without deadline/worker was accepted")
	}
	if _, err := store.DB().ExecContext(ctx, `
        INSERT INTO publication_outbox(post_id, job_id, state, created_at, updated_at)
        VALUES ('post-one', 'job-one', 'published', ?, ?)`, now, now); err == nil {
		t.Fatal("published outbox entry without commit SHA was accepted")
	}
}
