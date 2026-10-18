package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type SourceCursor struct {
	SourceID      string
	ETag          string
	LastModified  string
	LastCheckedAt time.Time
	ErrorCount    int
	LastError     string
}

func (s *Store) SourceCursor(ctx context.Context, sourceID string) (SourceCursor, bool, error) {
	if strings.TrimSpace(sourceID) == "" {
		return SourceCursor{}, false, errors.New("source_id is required")
	}
	var cursor SourceCursor
	var checkedAt string
	var etag, lastModified, lastError sql.NullString
	err := s.db.QueryRowContext(ctx, `
        SELECT source_id, etag, last_modified, last_checked_at, error_count, last_error
        FROM ingestion_cursors WHERE source_id = ?`, sourceID).
		Scan(&cursor.SourceID, &etag, &lastModified, &checkedAt, &cursor.ErrorCount, &lastError)
	if errors.Is(err, sql.ErrNoRows) {
		return SourceCursor{}, false, nil
	}
	if err != nil {
		return SourceCursor{}, false, fmt.Errorf("read source cursor: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, checkedAt)
	if err != nil {
		return SourceCursor{}, false, fmt.Errorf("parse source cursor timestamp: %w", err)
	}
	cursor.ETag, cursor.LastModified = etag.String, lastModified.String
	cursor.LastCheckedAt, cursor.LastError = parsed.UTC(), lastError.String
	return cursor, true, nil
}

func (s *Store) AdvanceSourceCursor(ctx context.Context, sourceID, etag, lastModified string, checkedAt time.Time) error {
	if strings.TrimSpace(sourceID) == "" {
		return errors.New("source_id is required")
	}
	stamp := checkedAt.UTC().Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx, `
        INSERT INTO ingestion_cursors(
            source_id, etag, last_modified, last_checked_at, error_count, last_error
        ) VALUES (?, ?, ?, ?, 0, NULL)
        ON CONFLICT(source_id) DO UPDATE SET
            etag = excluded.etag,
            last_modified = excluded.last_modified,
            last_checked_at = excluded.last_checked_at,
            error_count = 0,
            last_error = NULL`,
		sourceID, nullable(etag), nullable(lastModified), stamp); err != nil {
		return fmt.Errorf("advance source cursor: %w", err)
	}
	return nil
}

func (s *Store) RecordSourceFailure(ctx context.Context, sourceID string, checkedAt time.Time, message string) error {
	if strings.TrimSpace(sourceID) == "" {
		return errors.New("source_id is required")
	}
	if len(message) > 500 {
		message = message[:500]
	}
	if _, err := s.db.ExecContext(ctx, `
        INSERT INTO ingestion_cursors(
            source_id, etag, last_modified, last_checked_at, error_count, last_error
        ) VALUES (?, NULL, NULL, ?, 1, ?)
        ON CONFLICT(source_id) DO UPDATE SET
            last_checked_at = excluded.last_checked_at,
            error_count = ingestion_cursors.error_count + 1,
            last_error = excluded.last_error`,
		sourceID, checkedAt.UTC().Format(time.RFC3339Nano), message); err != nil {
		return fmt.Errorf("record source failure: %w", err)
	}
	return nil
}

func nullable(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
