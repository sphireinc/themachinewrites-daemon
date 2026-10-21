package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestPredictionReviewHistoryPersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "runtime.sqlite")
	reviewedAt := time.Date(2026, 10, 21, 14, 0, 0, 0, time.UTC)
	review := PredictionReview{
		CycleID: "cycle-review-1", PredictionID: "01M589K0G00000000000000000", Result: "unresolved",
		Assessment: "The available evidence does not yet determine whether the forecast was met.",
		Sources:    []PredictionReviewSource{{Title: "Review evidence", URL: "https://example.org/review", RetrievedAt: reviewedAt.Format(time.RFC3339)}},
		ReviewedAt: reviewedAt,
	}
	first, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.RecordPredictionReview(ctx, review); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	reviews, err := second.RecentPredictionReviews(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) != 1 || reviews[0].PredictionID != review.PredictionID || reviews[0].Result != review.Result || reviews[0].Assessment != review.Assessment || !reviews[0].ReviewedAt.Equal(reviewedAt) || len(reviews[0].Sources) != 1 {
		t.Fatalf("persisted prediction review = %#v", reviews)
	}
}

func TestPredictionReviewRejectsIncompleteAndDuplicateCycleRecords(t *testing.T) {
	ctx := context.Background()
	runtime, err := Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	at := time.Date(2026, 10, 21, 14, 0, 0, 0, time.UTC)
	valid := PredictionReview{
		CycleID: "cycle-review-1", PredictionID: "01M589K0G00000000000000000", Result: "supported", Assessment: "The measured result supports the forecast.",
		Sources: []PredictionReviewSource{{Title: "Evidence", URL: "https://example.org/evidence", RetrievedAt: at.Format(time.RFC3339)}}, ReviewedAt: at,
	}
	invalid := valid
	invalid.Result = "certain"
	if err := runtime.RecordPredictionReview(ctx, invalid); err == nil {
		t.Fatal("accepted unknown prediction result")
	}
	if err := runtime.RecordPredictionReview(ctx, valid); err != nil {
		t.Fatal(err)
	}
	if err := runtime.RecordPredictionReview(ctx, valid); err == nil {
		t.Fatal("accepted duplicate review in one editorial cycle")
	}
}
