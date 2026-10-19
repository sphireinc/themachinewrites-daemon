package memory

import (
	"sort"
	"strings"
	"time"
)

type RevisitSuggestion struct {
	PostID   string
	Title    string
	Reason   string
	DueAt    time.Time
	Disputed bool
	Priority int
}

type RevisitInput struct {
	PostID       string
	Title        string
	PredictionAt time.Time
	DueAt        time.Time
	Disputed     bool
}

func BuildRevisitQueue(items []RevisitInput, now time.Time) []RevisitSuggestion {
	seen := map[string]bool{}
	result := make([]RevisitSuggestion, 0)
	for _, item := range items {
		if seen[item.PostID] || strings.TrimSpace(item.PostID) == "" || strings.TrimSpace(item.Title) == "" {
			continue
		}
		due := item.DueAt
		if due.IsZero() {
			due = item.PredictionAt.Add(30 * 24 * time.Hour)
		}
		reason := ""
		priority := 0
		switch {
		case item.Disputed:
			reason = "active_dispute"
			priority = 2
		case !due.After(now):
			reason = "prediction_due"
			priority = 1
		default:
			continue
		}
		seen[item.PostID] = true
		result = append(result, RevisitSuggestion{PostID: item.PostID, Title: item.Title, Reason: reason, DueAt: due.UTC(), Disputed: item.Disputed, Priority: priority})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority > result[j].Priority
		}
		if !result[i].DueAt.Equal(result[j].DueAt) {
			return result[i].DueAt.Before(result[j].DueAt)
		}
		return result[i].PostID < result[j].PostID
	})
	if len(result) > 20 {
		result = result[:20]
	}
	return result
}
