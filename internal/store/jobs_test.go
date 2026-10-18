package store

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
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

func TestClaimJobIsExclusiveAndRecoversExpiredLease(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	if _, err := store.DB().ExecContext(ctx, `
        INSERT INTO jobs(job_id, state, created_at, updated_at)
        VALUES ('job-claim', 'pending', ?, ?)`, now.Format(time.RFC3339), now.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}

	results := make(chan ClaimResult, 2)
	var wg sync.WaitGroup
	for _, worker := range []string{"worker-a", "worker-b"} {
		wg.Add(1)
		go func(worker string) {
			defer wg.Done()
			claim, err := store.ClaimJob(ctx, worker, now, time.Minute)
			results <- ClaimResult{Claim: claim, Err: err}
		}(worker)
	}
	wg.Wait()
	close(results)

	claimed := 0
	var owner string
	for result := range results {
		if result.Err != nil {
			t.Fatal(result.Err)
		}
		if result.Claim.Found {
			claimed++
			owner = result.Claim.WorkerID
		}
	}
	if claimed != 1 {
		t.Fatalf("claimed by %d workers, want exactly 1", claimed)
	}

	claim, err := store.ClaimJob(ctx, "worker-c", now.Add(2*time.Minute), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !claim.Found || claim.JobID != "job-claim" || claim.WorkerID != "worker-c" {
		t.Fatalf("expired lease claim = %#v, want worker-c takeover from %s", claim, owner)
	}
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
