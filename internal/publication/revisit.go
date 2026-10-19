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
	revisited := make(map[string]bool)
	for _, post := range posts {
		if post.Type == "revisit" || post.Type == "correction" {
			for _, related := range post.RelatedIDs {
				revisited[related] = true
			}
			if post.CorrectionOf != "" {
				revisited[post.CorrectionOf] = true
			}
		}
	}
	var result []RevisitCandidate
	for _, post := range posts {
		if post.Type == "prediction" && !revisited[post.ID] && now.Sub(post.PublishedAt) >= minAge {
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
