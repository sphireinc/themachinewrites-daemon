package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestAcknowledgePublicationIsIdempotentForSameSHAAndRejectsDifferentSHA(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	const now = "2026-10-18T20:00:00Z"
	if _, err := store.DB().ExecContext(ctx, `
        INSERT INTO jobs(job_id, state, candidate_path, post_id, created_at, updated_at)
        VALUES ('job-publish', 'validated', 'candidates/job-publish.json', 'post-one', ?, ?)`,
		now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `
        INSERT INTO publication_outbox(post_id, job_id, state, created_at, updated_at)
        VALUES ('post-one', 'job-publish', 'publishing', ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}

	if err := store.AcknowledgePublication(ctx, "post-one", "sha-one", now); err != nil {
		t.Fatalf("first acknowledgement error = %v", err)
	}
	if err := store.AcknowledgePublication(ctx, "post-one", "sha-one", now); err != nil {
		t.Fatalf("repeat acknowledgement error = %v", err)
	}
	err = store.AcknowledgePublication(ctx, "post-one", "sha-two", now)
	if !errors.Is(err, ErrPublicationConflict) {
		t.Fatalf("mismatched acknowledgement error = %v, want ErrPublicationConflict", err)
	}

	var jobState, jobSHA, outboxState, outboxSHA string
	if err := store.DB().QueryRowContext(ctx, `
        SELECT j.state, j.commit_sha, o.state, o.commit_sha
        FROM jobs j JOIN publication_outbox o USING(job_id)
        WHERE j.job_id = 'job-publish'`).Scan(&jobState, &jobSHA, &outboxState, &outboxSHA); err != nil {
		t.Fatal(err)
	}
	if jobState != "published" || outboxState != "published" || jobSHA != "sha-one" || outboxSHA != "sha-one" {
		t.Fatalf("stored state = %s/%s %s/%s", jobState, outboxState, jobSHA, outboxSHA)
	}
}
