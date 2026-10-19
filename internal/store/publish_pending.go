package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Store) SavePendingPublication(ctx context.Context, postID string) error {
	if strings.TrimSpace(postID) == "" {
		return errors.New("post ID required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	jobID := "publish-" + postID
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO jobs(job_id,state,post_id,created_at,updated_at)
		VALUES (?, 'validated', ?, ?, ?)
		ON CONFLICT(job_id) DO UPDATE SET state='validated', post_id=excluded.post_id, updated_at=excluded.updated_at`,
		jobID, postID, now, now); err != nil {
		return fmt.Errorf("save pending job: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
        INSERT INTO publication_outbox(post_id,job_id,state,created_at,updated_at)
        VALUES (?,?,'pending',?,?)
        ON CONFLICT(post_id) DO UPDATE SET state='pending', updated_at=excluded.updated_at`,
		postID, jobID, now, now); err != nil {
		return fmt.Errorf("save pending outbox: %w", err)
	}
	return tx.Commit()
}

func (s *Store) AcknowledgePendingPublication(ctx context.Context, postID, commitSHA string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return s.AcknowledgePublication(ctx, postID, commitSHA, now)
}

func (s *Store) SavePending(ctx context.Context, postID string) error {
	return s.SavePendingPublication(ctx, postID)
}

func (s *Store) Acknowledge(ctx context.Context, postID, commitSHA string) error {
	return s.AcknowledgePendingPublication(ctx, postID, commitSHA)
}
