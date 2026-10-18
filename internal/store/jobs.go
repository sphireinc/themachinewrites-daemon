package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type JobClaim struct {
	Found         bool
	JobID         string
	WorkerID      string
	Attempts      int
	LeaseDeadline time.Time
}

type ClaimResult struct {
	Claim JobClaim
	Err   error
}

func (s *Store) ClaimJob(ctx context.Context, workerID string, now time.Time, lease time.Duration) (JobClaim, error) {
	if strings.TrimSpace(workerID) == "" {
		return JobClaim{}, errors.New("worker_id is required")
	}
	if lease <= 0 {
		return JobClaim{}, errors.New("lease duration must be greater than zero")
	}
	now = now.UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return JobClaim{}, fmt.Errorf("begin job claim: %w", err)
	}
	defer tx.Rollback()

	var jobID string
	var attempts int
	err = tx.QueryRowContext(ctx, `
        SELECT job_id, attempts
        FROM jobs
        WHERE state IN ('pending', 'leased')
          AND (state = 'pending' OR lease_deadline IS NULL OR lease_deadline <= ?)
        ORDER BY created_at ASC, job_id ASC
        LIMIT 1`, now.Format(time.RFC3339Nano)).Scan(&jobID, &attempts)
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.Commit(); err != nil {
			return JobClaim{}, fmt.Errorf("commit empty job claim: %w", err)
		}
		return JobClaim{}, nil
	}
	if err != nil {
		return JobClaim{}, fmt.Errorf("select claimable job: %w", err)
	}
	deadline := now.Add(lease)
	result, err := tx.ExecContext(ctx, `
        UPDATE jobs
        SET state = 'leased', attempts = attempts + 1,
            lease_deadline = ?, worker_id = ?, updated_at = ?
        WHERE job_id = ?
          AND (state = 'pending' OR lease_deadline IS NULL OR lease_deadline <= ?)`,
		deadline.Format(time.RFC3339Nano), workerID, now.Format(time.RFC3339Nano), jobID, now.Format(time.RFC3339Nano))
	if err != nil {
		return JobClaim{}, fmt.Errorf("lease job %s: %w", jobID, err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return JobClaim{}, fmt.Errorf("read lease result: %w", err)
	}
	if updated != 1 {
		return JobClaim{}, errors.New("job lease changed concurrently")
	}
	if err := tx.Commit(); err != nil {
		return JobClaim{}, fmt.Errorf("commit job claim: %w", err)
	}
	return JobClaim{Found: true, JobID: jobID, WorkerID: workerID, Attempts: attempts + 1, LeaseDeadline: deadline}, nil
}
