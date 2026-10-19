package publication

import (
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"strings"
	"testing"
	"time"
)

func TestRevisitCandidatesSelectMaturedPredictions(t *testing.T) {
	now := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	posts := []PostRecord{
		{ID: "01M589K0G00000000000000000", Type: "prediction", PublishedAt: now.Add(-40 * 24 * time.Hour), Title: "Old prediction"},
		{ID: "01M589K0G00000000000000001", Type: "prediction", PublishedAt: now.Add(-2 * 24 * time.Hour), Title: "Recent prediction"},
		{ID: "01M589K0G00000000000000002", Type: "observation", PublishedAt: now.Add(-50 * 24 * time.Hour), Title: "Observation"},
		{ID: "01M589K0G00000000000000003", Type: "correction", PublishedAt: now.Add(-time.Hour), CorrectionOf: "01M589K0G00000000000000000", Title: "Reviewed already"},
	}
	candidates := BuildRevisitCandidates(posts, now, 30*24*time.Hour)
	if len(candidates) != 0 {
		t.Fatalf("candidates=%#v", candidates)
	}
	candidates = BuildRevisitCandidates(posts[:2], now, 30*24*time.Hour)
	if len(candidates) != 1 || candidates[0].ID != posts[0].ID {
		t.Fatalf("unreviewed candidates=%#v", candidates)
	}
}

func TestRevisitPromptKeepsDeferAvailable(t *testing.T) {
	candidate := RevisitCandidate{ID: "01M589K0G00000000000000000", Title: "Prediction", PublishedAt: time.Now().UTC()}
	prompt := RevisitPrompt([]RevisitCandidate{candidate})
	if prompt == "" {
		t.Fatal("empty prompt")
	}
	if strings.Contains(prompt, "publish a revisit") || !strings.Contains(prompt, "correction linked") || !strings.Contains(prompt, candidate.ID) {
		t.Fatalf("revisit prompt is incompatible with v1 contract: %s", prompt)
	}
	_ = engine.ActionDefer
}
