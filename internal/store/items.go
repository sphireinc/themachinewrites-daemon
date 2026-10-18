package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type ObservedItem struct {
	URLCanonicalHash string
	SourceID         string
	DiscoveredAt     string
	Fingerprint      string
	LastSeenAt       string
}

func (s *Store) ObserveItem(ctx context.Context, item ObservedItem) (bool, error) {
	if strings.TrimSpace(item.URLCanonicalHash) == "" ||
		strings.TrimSpace(item.SourceID) == "" ||
		strings.TrimSpace(item.Fingerprint) == "" {
		return false, errors.New("canonical URL hash, source_id and fingerprint are required")
	}
	discoveredAt, err := normaliseTimestamp(item.DiscoveredAt)
	if err != nil {
		return false, fmt.Errorf("discovered_at: %w", err)
	}
	lastSeenAt, err := normaliseTimestamp(item.LastSeenAt)
	if err != nil {
		return false, fmt.Errorf("last_seen_at: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin observed-item transaction: %w", err)
	}
	defer tx.Rollback()

	var existing int
	if err := tx.QueryRowContext(ctx,
		"SELECT count(*) FROM observed_items WHERE fingerprint = ?", item.Fingerprint).Scan(&existing); err != nil {
		return false, fmt.Errorf("check observed item: %w", err)
	}
	if existing == 1 {
		if _, err := tx.ExecContext(ctx,
			"UPDATE observed_items SET last_seen_at = ? WHERE fingerprint = ?",
			lastSeenAt, item.Fingerprint); err != nil {
			return false, fmt.Errorf("refresh observed item: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit observed-item refresh: %w", err)
		}
		return false, nil
	}

	if _, err := tx.ExecContext(ctx, `
        INSERT INTO observed_items(
            url_canonical_hash, source_id, discovered_at, fingerprint, last_seen_at
        ) VALUES (?, ?, ?, ?, ?)`,
		item.URLCanonicalHash, item.SourceID, discoveredAt, item.Fingerprint, lastSeenAt); err != nil {
		return false, fmt.Errorf("record observed item: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit observed item: %w", err)
	}
	return true, nil
}

func normaliseTimestamp(value string) (string, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return "", errors.New("must be an RFC3339 UTC timestamp")
	}
	if parsed.Location() != time.UTC {
		return "", errors.New("must use UTC")
	}
	return parsed.UTC().Format(time.RFC3339Nano), nil
}
