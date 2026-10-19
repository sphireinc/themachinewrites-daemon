package publication

import (
	"encoding/json"
	"sort"
	"time"
)

type RevisitCandidate struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	PublishedAt time.Time `json:"published_at"`
}

func BuildRevisitCandidates(posts []PostRecord, now time.Time, minAge time.Duration) []RevisitCandidate {
	var result []RevisitCandidate
	for _, post := range posts {
		if post.Type == "prediction" && now.Sub(post.PublishedAt) >= minAge {
			result = append(result, RevisitCandidate{ID: post.ID, Title: post.Title, PublishedAt: post.PublishedAt})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].PublishedAt.Before(result[j].PublishedAt) })
	if len(result) > 20 {
		result = result[:20]
	}
	return result
}

func RevisitPrompt(candidates []RevisitCandidate) string {
	raw, _ := json.Marshal(candidates)
	return "Review these matured predictions and choose investigate, defer, publish a revisit, or publish a correction: " + string(raw)
}
