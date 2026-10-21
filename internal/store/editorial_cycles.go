package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type EditorialCycle struct {
	ID                string
	RequestedMode     string
	SelectedMode      string
	EligibilityReason string
	EligibleModes     []string
	Topics            []string
	Outcome           string
	PublicationID     string
	SelectedAt        time.Time
	CompletedAt       time.Time
}

func (s *Store) RecordEditorialCycle(ctx context.Context, cycle EditorialCycle) error {
	if strings.TrimSpace(cycle.ID) == "" || strings.TrimSpace(cycle.RequestedMode) == "" || cycle.SelectedAt.IsZero() {
		return errors.New("editorial cycle ID, requested mode, and selected time are required")
	}
	if cycle.Outcome == "" {
		cycle.Outcome = "pending"
	}
	if !validEditorialOutcome(cycle.Outcome) {
		return fmt.Errorf("invalid editorial cycle outcome %q", cycle.Outcome)
	}
	eligible, err := json.Marshal(nonNilStrings(cycle.EligibleModes))
	if err != nil {
		return fmt.Errorf("encode eligible modes: %w", err)
	}
	topics, err := json.Marshal(nonNilStrings(cycle.Topics))
	if err != nil {
		return fmt.Errorf("encode editorial topics: %w", err)
	}
	var completed any
	if !cycle.CompletedAt.IsZero() {
		completed = cycle.CompletedAt.UTC().Format(time.RFC3339Nano)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO editorial_cycles(
		cycle_id,requested_mode,selected_mode,eligibility_reason,eligible_modes,topics,outcome,publication_id,selected_at,completed_at
	) VALUES(?,?,?,?,?,?,?,?,?,?)`, cycle.ID, cycle.RequestedMode, cycle.SelectedMode, cycle.EligibilityReason,
		string(eligible), string(topics), cycle.Outcome, nullableString(cycle.PublicationID), cycle.SelectedAt.UTC().Format(time.RFC3339Nano), completed)
	if err != nil {
		return fmt.Errorf("record editorial cycle: %w", err)
	}
	return nil
}

func (s *Store) FinishEditorialCycle(ctx context.Context, id, outcome, publicationID string, completedAt time.Time) error {
	if strings.TrimSpace(id) == "" || !validEditorialOutcome(outcome) || outcome == "pending" || completedAt.IsZero() {
		return errors.New("editorial cycle completion requires an ID, terminal outcome, and completion time")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE editorial_cycles SET outcome=?,publication_id=?,completed_at=? WHERE cycle_id=? AND outcome='pending'`,
		outcome, nullableString(publicationID), completedAt.UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("finish editorial cycle: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated != 1 {
		return fmt.Errorf("editorial cycle %q is missing or already complete", id)
	}
	return nil
}

func (s *Store) UpdateEditorialCycleTopics(ctx context.Context, id string, topics []string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("editorial cycle ID is required")
	}
	encoded, err := json.Marshal(nonNilStrings(topics))
	if err != nil {
		return fmt.Errorf("encode editorial topics: %w", err)
	}
	result, err := s.db.ExecContext(ctx, `UPDATE editorial_cycles SET topics=? WHERE cycle_id=?`, string(encoded), id)
	if err != nil {
		return fmt.Errorf("update editorial topics: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("editorial cycle %q does not exist", id)
	}
	return nil
}

func (s *Store) RecentEditorialCycles(ctx context.Context, limit int) ([]EditorialCycle, error) {
	if limit < 1 || limit > 10000 {
		return nil, errors.New("editorial cycle history limit must be between 1 and 10000")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT cycle_id,requested_mode,selected_mode,eligibility_reason,eligible_modes,topics,outcome,COALESCE(publication_id,''),selected_at,COALESCE(completed_at,'')
		FROM editorial_cycles ORDER BY selected_at DESC,cycle_id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("read editorial cycle history: %w", err)
	}
	defer rows.Close()
	cycles := make([]EditorialCycle, 0, limit)
	for rows.Next() {
		var cycle EditorialCycle
		var eligibleRaw, topicsRaw, selectedAt, completedAt string
		if err := rows.Scan(&cycle.ID, &cycle.RequestedMode, &cycle.SelectedMode, &cycle.EligibilityReason, &eligibleRaw, &topicsRaw,
			&cycle.Outcome, &cycle.PublicationID, &selectedAt, &completedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(eligibleRaw), &cycle.EligibleModes); err != nil {
			return nil, fmt.Errorf("decode eligible modes for cycle %q: %w", cycle.ID, err)
		}
		if err := json.Unmarshal([]byte(topicsRaw), &cycle.Topics); err != nil {
			return nil, fmt.Errorf("decode topics for cycle %q: %w", cycle.ID, err)
		}
		cycle.SelectedAt, err = time.Parse(time.RFC3339Nano, selectedAt)
		if err != nil {
			return nil, fmt.Errorf("parse selected time for cycle %q: %w", cycle.ID, err)
		}
		if completedAt != "" {
			cycle.CompletedAt, err = time.Parse(time.RFC3339Nano, completedAt)
			if err != nil {
				return nil, fmt.Errorf("parse completed time for cycle %q: %w", cycle.ID, err)
			}
		}
		cycles = append(cycles, cycle)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cycles, nil
}

func (s *Store) PruneEditorialCycles(ctx context.Context, retain int) error {
	if retain < 1 || retain > 10000 {
		return errors.New("editorial cycle retention must be between 1 and 10000")
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM editorial_cycles
		WHERE cycle_id NOT IN (
			SELECT cycle_id FROM editorial_cycles ORDER BY selected_at DESC,cycle_id DESC LIMIT ?
		)`, retain)
	if err != nil {
		return fmt.Errorf("prune editorial cycle history: %w", err)
	}
	return nil
}

func validEditorialOutcome(outcome string) bool {
	switch outcome {
	case "pending", "published", "deferred", "investigated", "dry_run", "no_eligible_mode", "failed":
		return true
	default:
		return false
	}
}

func nullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
