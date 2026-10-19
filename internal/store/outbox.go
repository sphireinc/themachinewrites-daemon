package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrPublicationConflict = errors.New("publication acknowledgement conflicts with confirmed commit")

func (s *Store) AcknowledgePublication(ctx context.Context, postID, commitSHA, at string) error {
	if strings.TrimSpace(postID) == "" || strings.TrimSpace(commitSHA) == "" {
		return errors.New("post_id and commit_sha are required")
	}
	timestamp, err := normaliseTimestamp(at)
	if err != nil {
		return fmt.Errorf("acknowledged_at: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin publication acknowledgement: %w", err)
	}
	defer tx.Rollback()

	var jobID, outboxState string
	var existing sql.NullString
	if err := tx.QueryRowContext(ctx, `
        SELECT o.job_id, o.state, o.commit_sha
        FROM publication_outbox o
        WHERE o.post_id = ?`, postID).Scan(&jobID, &outboxState, &existing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("publication outbox entry not found for %s", postID)
		}
		return fmt.Errorf("read publication outbox: %w", err)
	}
	if existing.Valid {
		if existing.String != commitSHA {
			return ErrPublicationConflict
		}
		if _, err := tx.ExecContext(ctx, `
            UPDATE jobs
            SET state = 'published', commit_sha = ?, lease_deadline = NULL,
                worker_id = NULL, last_error = NULL, updated_at = ?
            WHERE job_id = ? AND post_id = ?`,
			commitSHA, timestamp, jobID, postID); err != nil {
			return fmt.Errorf("reconcile published job: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit idempotent acknowledgement: %w", err)
		}
		return nil
	}
	if outboxState == "published" {
		return errors.New("published outbox entry has no commit SHA")
	}
	if _, err := tx.ExecContext(ctx, `
        UPDATE publication_outbox
        SET state = 'published', commit_sha = ?, updated_at = ?
        WHERE post_id = ? AND state IN ('pending', 'publishing', 'failed')`,
		commitSHA, timestamp, postID); err != nil {
		return fmt.Errorf("update publication outbox: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
        UPDATE jobs
        SET state = 'published', commit_sha = ?, lease_deadline = NULL,
            worker_id = NULL, last_error = NULL, updated_at = ?
        WHERE job_id = ? AND post_id = ?`,
		commitSHA, timestamp, jobID, postID); err != nil {
		return fmt.Errorf("update published job: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit publication acknowledgement: %w", err)
	}
	return nil
}

func (s *Store) RestorePublicationAcknowledgement(ctx context.Context, postID, commitSHA string) error {
	if strings.TrimSpace(postID) == "" || strings.TrimSpace(commitSHA) == "" {
		return errors.New("post_id and commit_sha are required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	jobID := "publish-" + postID
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin publication restoration: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO jobs(job_id, state, post_id, created_at, updated_at)
		VALUES (?, 'published', ?, ?, ?)
		ON CONFLICT(job_id) DO NOTHING`,
		jobID, postID, now, now); err != nil {
		return fmt.Errorf("restore publication job: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO publication_outbox(post_id, job_id, state, commit_sha, created_at, updated_at)
		VALUES (?, ?, 'published', ?, ?, ?)
		ON CONFLICT(post_id) DO NOTHING`,
		postID, jobID, commitSHA, now, now); err != nil {
		return fmt.Errorf("restore publication outbox: %w", err)
	}

	var storedJobID, state string
	var existing sql.NullString
	if err := tx.QueryRowContext(ctx, `
		SELECT job_id, state, commit_sha FROM publication_outbox WHERE post_id = ?`,
		postID).Scan(&storedJobID, &state, &existing); err != nil {
		return fmt.Errorf("read restored publication: %w", err)
	}
	if existing.Valid && existing.String != "" && existing.String != commitSHA {
		return ErrPublicationConflict
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE publication_outbox
		SET state = 'published', commit_sha = ?, updated_at = ?
		WHERE post_id = ?`,
		commitSHA, now, postID); err != nil {
		return fmt.Errorf("mark restored outbox published: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE jobs
		SET state = 'published', commit_sha = ?, lease_deadline = NULL,
		    worker_id = NULL, last_error = NULL, updated_at = ?
		WHERE job_id = ? AND post_id = ?`,
		commitSHA, now, storedJobID, postID); err != nil {
		return fmt.Errorf("mark restored job published: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit publication restoration: %w", err)
	}
	return nil
}

func (s *Store) OutstandingPublicationIDs(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT post_id FROM jobs WHERE post_id IS NOT NULL
		UNION
		SELECT post_id FROM publication_outbox`)
	if err != nil {
		return nil, fmt.Errorf("read local publication IDs: %w", err)
	}
	defer rows.Close()
	result := map[string]bool{}
	for rows.Next() {
		var postID string
		if err := rows.Scan(&postID); err != nil {
			return nil, fmt.Errorf("scan local publication ID: %w", err)
		}
		postID = strings.TrimSpace(postID)
		if len(postID) != 26 {
			return nil, fmt.Errorf("invalid local publication ID %q", postID)
		}
		result[postID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate local publication IDs: %w", err)
	}
	return result, nil
}
