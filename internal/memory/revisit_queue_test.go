package memory

import (
	"testing"
	"time"
)

func TestRevisitQueueSchedulesDuePredictionWithoutForcingPost(t *testing.T) {
	now := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	items := []RevisitInput{
		{PostID: "due", Title: "Due prediction", PredictionAt: now.Add(-40 * 24 * time.Hour)},
		{PostID: "future", Title: "Future prediction", DueAt: now.Add(24 * time.Hour)},
		{PostID: "disputed", Title: "Disputed claim", DueAt: now.Add(24 * time.Hour), Disputed: true},
	}
	queue := BuildRevisitQueue(items, now)
	if len(queue) != 2 || queue[0].PostID != "disputed" || queue[1].Reason != "prediction_due" {
		t.Fatalf("queue = %#v", queue)
	}
}
