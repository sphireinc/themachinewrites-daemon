package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type PredictionReviewSource struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	RetrievedAt string `json:"retrieved_at"`
}

type PredictionReview struct {
	CycleID      string
	PredictionID string
	Result       string
	Assessment   string
	Sources      []PredictionReviewSource
	ReviewedAt   time.Time
}

func (s *Store) RecordPredictionReview(ctx context.Context, review PredictionReview) error {
	return s.RecordPredictionReviews(ctx, []PredictionReview{review})
}

func (s *Store) RecordPredictionReviews(ctx context.Context, reviews []PredictionReview) error {
	if len(reviews) > 10 {
		return errors.New("an editorial cycle may record at most 10 prediction reviews")
	}
	if len(reviews) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin prediction review history: %w", err)
	}
	defer tx.Rollback()
	for _, review := range reviews {
		if err := validatePredictionReview(review); err != nil {
			return err
		}
		sources, err := json.Marshal(review.Sources)
		if err != nil {
			return fmt.Errorf("encode prediction review sources: %w", err)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO prediction_reviews(cycle_id,prediction_id,result,assessment,sources,reviewed_at) VALUES(?,?,?,?,?,?)`,
			review.CycleID, review.PredictionID, review.Result, strings.TrimSpace(review.Assessment), string(sources), review.ReviewedAt.UTC().Format(time.RFC3339Nano))
		if err != nil {
			return fmt.Errorf("record prediction review: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit prediction review history: %w", err)
	}
	return nil
}

func validatePredictionReview(review PredictionReview) error {
	if strings.TrimSpace(review.CycleID) == "" || strings.TrimSpace(review.PredictionID) == "" || review.ReviewedAt.IsZero() {
		return errors.New("prediction review requires a cycle, target prediction, and review time")
	}
	switch review.Result {
	case "supported", "partially_supported", "not_supported", "unresolved":
	default:
		return fmt.Errorf("invalid prediction review result %q", review.Result)
	}
	if len([]rune(strings.TrimSpace(review.Assessment))) == 0 || len([]rune(review.Assessment)) > 2000 {
		return errors.New("prediction review assessment must contain 1–2000 characters")
	}
	if len(review.Sources) == 0 || len(review.Sources) > 10 {
		return errors.New("prediction review requires 1–10 evidence sources")
	}
	for index, source := range review.Sources {
		parsed, err := url.Parse(source.URL)
		if strings.TrimSpace(source.Title) == "" || len([]rune(source.Title)) > 240 || len(source.URL) > 2048 || err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
			return fmt.Errorf("prediction review source %d must have a title and absolute HTTPS URL", index+1)
		}
		if _, err := time.Parse(time.RFC3339Nano, source.RetrievedAt); err != nil {
			return fmt.Errorf("prediction review source %d retrieved_at must be RFC3339", index+1)
		}
	}
	return nil
}

func (s *Store) RecentPredictionReviews(ctx context.Context, limit int) ([]PredictionReview, error) {
	if limit < 1 || limit > 10000 {
		return nil, errors.New("prediction review history limit must be between 1 and 10000")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT cycle_id,prediction_id,result,assessment,sources,reviewed_at FROM prediction_reviews ORDER BY reviewed_at DESC,rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("read prediction review history: %w", err)
	}
	defer rows.Close()
	result := make([]PredictionReview, 0, limit)
	for rows.Next() {
		var review PredictionReview
		var sources, reviewedAt string
		if err := rows.Scan(&review.CycleID, &review.PredictionID, &review.Result, &review.Assessment, &sources, &reviewedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(sources), &review.Sources); err != nil {
			return nil, fmt.Errorf("decode prediction review sources for %q: %w", review.PredictionID, err)
		}
		review.ReviewedAt, err = time.Parse(time.RFC3339Nano, reviewedAt)
		if err != nil {
			return nil, fmt.Errorf("parse prediction review time for %q: %w", review.PredictionID, err)
		}
		result = append(result, review)
	}
	if err := rows.Err(); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return result, nil
}
