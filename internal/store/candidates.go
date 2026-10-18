package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Candidate struct {
	URLCanonicalHash string
	URL              string
	SourceID         string
	SourceName       string
	Category         string
	Headline         string
	PublishedAt      string
	Excerpt          string
	DiscoveredAt     string
	Fingerprint      string
	LastSeenAt       string
}

func (s *Store) UpsertCandidate(ctx context.Context, candidate Candidate) (bool, error) {
	if strings.TrimSpace(candidate.Fingerprint) == "" || strings.TrimSpace(candidate.URL) == "" {
		return false, errors.New("candidate URL and fingerprint are required")
	}
	for name, value := range map[string]string{
		"published_at":  candidate.PublishedAt,
		"discovered_at": candidate.DiscoveredAt,
		"last_seen_at":  candidate.LastSeenAt,
	} {
		if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
			return false, fmt.Errorf("%s must be RFC3339: %w", name, err)
		}
	}
	inserted, err := s.ObserveItem(ctx, ObservedItem{
		URLCanonicalHash: candidate.URLCanonicalHash,
		SourceID:         candidate.SourceID,
		DiscoveredAt:     candidate.DiscoveredAt,
		Fingerprint:      candidate.Fingerprint,
		LastSeenAt:       candidate.LastSeenAt,
	})
	if err != nil || !inserted {
		return inserted, err
	}
	if _, err := s.db.ExecContext(ctx, `
        UPDATE observed_items SET
            headline = ?, source_name = ?, category = ?, published_at = ?,
            excerpt = ?, retrieved_at = ?, url = ?
        WHERE fingerprint = ?`,
		candidate.Headline, candidate.SourceName, candidate.Category, candidate.PublishedAt,
		candidate.Excerpt, candidate.DiscoveredAt, candidate.URL, candidate.Fingerprint); err != nil {
		return false, fmt.Errorf("store candidate details: %w", err)
	}
	return true, nil
}
