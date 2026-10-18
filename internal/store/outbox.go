package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
