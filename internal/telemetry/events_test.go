package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestFixtureCycleEmitsStableOrderedLifecycleEvents(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, nil))
	emitter := NewEmitter(logger)
	events := []Event{EventSourcePoll, EventCandidate, EventModelStarted, EventModelDeferred, EventPostStaged, EventPushConfirmed}
	for _, event := range events {
		emitter.Emit(context.Background(), event, "job-42", 25*time.Millisecond)
	}
	lines := strings.Split(strings.TrimSpace(buffer.String()), "\n")
	if len(lines) != len(events) {
		t.Fatalf("event lines = %d", len(lines))
	}
	for index, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if record["event"] != string(events[index]) || record["job_id"] != "job-42" || record["duration_ms"] != float64(25) {
			t.Fatalf("record %d = %#v", index, record)
		}
	}
}

func TestCountersIncrementAndHistogramIsBounded(t *testing.T) {
	var metrics Metrics
	metrics.ModelInvoked()
	metrics.SourceErrored()
	metrics.Published()
	metrics.Retried()
	metrics.ObserveDurationMilliseconds(5)
	metrics.ObserveDurationMilliseconds(5000)
	snapshot := metrics.Snapshot()
	if snapshot.Invocations != 1 || snapshot.SourceErrors != 1 || snapshot.Publications != 1 || snapshot.Retries != 1 {
		t.Fatalf("counters = %#v", snapshot)
	}
	if len(snapshot.DurationHistogram) != histogramBuckets {
		t.Fatalf("histogram buckets = %d", len(snapshot.DurationHistogram))
	}
	metrics.Reset()
	if metrics.Snapshot().Invocations != 0 {
		t.Fatal("reset did not clear counters")
	}
}

func TestUnknownEventFailsClosedWithNoTopicCardinality(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, nil))
	NewEmitter(logger).Emit(context.Background(), Event("topic:arbitrary-value"), "job-1", 0)
	if !strings.Contains(buffer.String(), `"event":"cycle_failed"`) {
		t.Fatalf("unknown event not normalized: %s", buffer.String())
	}
}
