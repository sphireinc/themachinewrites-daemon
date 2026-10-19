package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestInfoWritesBoundedStructuredEvent(t *testing.T) {
	var buffer bytes.Buffer
	logger := New(&buffer)
	logger.Info("source fetched",
		"event", "source_fetch",
		"job_id", "job-42",
		"duration", 1500*time.Millisecond,
		"url", "https://user:password@example.org/feed.xml",
		"error", errors.New(strings.Repeat("failure detail ", 100)),
	)

	output := buffer.String()
	for _, want := range []string{`"event":"source_fetch"`, `"job_id":"job-42"`, `"duration_ms":1500`, `"url":"https://example.org/feed.xml"`} {
		if !strings.Contains(output, want) {
			t.Errorf("output %q does not contain %q", output, want)
		}
	}
	if strings.Contains(output, "password") || strings.Contains(output, "user:") {
		t.Fatalf("output leaked URL credentials: %q", output)
	}
	if len(output) > 1200 {
		t.Fatalf("output length = %d, want bounded output", len(output))
	}
}

func TestSecretCanaryNeverAppearsInLogs(t *testing.T) {
	const canary = "log-secret-canary-8fcc731"
	var buffer bytes.Buffer
	logger := New(&buffer)
	logger.Error("publication failed", "event", "publication", "error", errors.New("token="+canary))

	if strings.Contains(buffer.String(), canary) {
		t.Fatalf("log leaked canary: %q", buffer.String())
	}
}

func TestNewUsesJSONAndConstrainsMessage(t *testing.T) {
	var buffer bytes.Buffer
	logger := New(&buffer)
	logger.Info(strings.Repeat("x", 1000), "event", "health")

	if !strings.HasPrefix(buffer.String(), "{") {
		t.Fatalf("log is not JSON: %q", buffer.String())
	}
	if len(buffer.String()) > 1200 {
		t.Fatalf("log length = %d, want bounded", len(buffer.String()))
	}
	var record map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &record); err != nil {
		t.Fatalf("decode structured log: %v; output = %q", err, buffer.String())
	}
	if record["event"] != "health" {
		t.Fatalf("event = %#v, want health", record["event"])
	}
}

func TestRetainedLogWriterIsBoundedAndStillRedacted(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewWithRetention(&buffer, 300)
	const canary = "retention-secret-canary"
	for range 20 {
		logger.Error("failure", "event", "cycle_failed", "url", "https://user:pass@example.org/feed", "error", "authorization="+canary)
	}
	if len(buffer.String()) > 300 {
		t.Fatalf("retained log length = %d", len(buffer.String()))
	}
	if strings.Contains(buffer.String(), canary) || strings.Contains(buffer.String(), "user:pass") {
		t.Fatalf("retained log leaked sensitive values: %q", buffer.String())
	}
}
