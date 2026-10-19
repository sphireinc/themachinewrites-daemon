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

func (s *Store) PendingCandidates(ctx context.Context, limit int) ([]Candidate, error) {
	if limit < 1 || limit > 100 {
		return nil, errors.New("candidate limit must be 1–100")
	}
	rows, err := s.db.QueryContext(ctx, `
        SELECT url_canonical_hash, url, source_id, source_name, category, headline,
               published_at, excerpt, discovered_at, fingerprint, last_seen_at
        FROM observed_items
        WHERE url <> '' AND headline <> '' AND decision_state = 'pending'
        ORDER BY published_at DESC, fingerprint ASC
        LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query candidates: %w", err)
	}
	defer rows.Close()
	var candidates []Candidate
	for rows.Next() {
		var candidate Candidate
		if err := rows.Scan(&candidate.URLCanonicalHash, &candidate.URL, &candidate.SourceID, &candidate.SourceName,
			&candidate.Category, &candidate.Headline, &candidate.PublishedAt, &candidate.Excerpt,
			&candidate.DiscoveredAt, &candidate.Fingerprint, &candidate.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate candidates: %w", err)
	}
	return candidates, nil
}

func (s *Store) MarkCandidatesEvaluating(ctx context.Context, fingerprints []string, at time.Time) error {
	if len(fingerprints) == 0 || len(fingerprints) > 100 {
		return errors.New("candidate fingerprint count must be 1–100")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin candidate decision: %w", err)
	}
	defer tx.Rollback()
	for _, fingerprint := range fingerprints {
		if strings.TrimSpace(fingerprint) == "" {
			return errors.New("candidate fingerprint is required")
		}
		result, err := tx.ExecContext(ctx, `UPDATE observed_items SET decision_state = 'evaluating', decision_updated_at = ? WHERE fingerprint = ? AND decision_state = 'pending'`, at.UTC().Format(time.RFC3339Nano), fingerprint)
		if err != nil {
			return fmt.Errorf("mark candidate evaluating: %w", err)
		}
		updated, err := result.RowsAffected()
		if err != nil || updated != 1 {
			return errors.New("candidate state changed concurrently")
		}
	}
	return tx.Commit()
}

func (s *Store) SetCandidateDecision(ctx context.Context, fingerprints []string, state string, at time.Time) error {
	if state != "deferred" && state != "published" && state != "failed" && state != "pending" {
		return errors.New("invalid candidate decision state")
	}
	if len(fingerprints) == 0 || len(fingerprints) > 100 {
		return errors.New("candidate fingerprint count must be 1–100")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin candidate result: %w", err)
	}
	defer tx.Rollback()
	for _, fingerprint := range fingerprints {
		if _, err := tx.ExecContext(ctx, `UPDATE observed_items SET decision_state = ?, decision_updated_at = ? WHERE fingerprint = ? AND decision_state = 'evaluating'`, state, at.UTC().Format(time.RFC3339Nano), fingerprint); err != nil {
			return fmt.Errorf("set candidate decision: %w", err)
		}
	}
	return tx.Commit()
}
