package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHealthSnapshotAtomicallyReplacesCompleteDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime", "health.json")
	first := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	if err := WriteHealth(path, Health{SchemaVersion: 1, LastPoll: &first}); err != nil {
		t.Fatal(err)
	}
	second := first.Add(time.Minute)
	if err := WriteHealth(path, Health{SchemaVersion: 1, LastPoll: &second, PausedReason: "budget_limit"}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(content, &value); err != nil {
		t.Fatalf("snapshot is not complete JSON: %v; %q", err, content)
	}
	loaded, err := ReadHealth(path)
	if err != nil || loaded.PausedReason != "budget_limit" || loaded.LastPoll == nil || !loaded.LastPoll.Equal(second) {
		t.Fatalf("loaded health = %#v, %v", loaded, err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("health mode = %v, %v", info.Mode().Perm(), err)
	}
}

func TestTruncatedSnapshotFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "health.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"last_poll":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadHealth(path); err == nil {
		t.Fatal("truncated JSON accepted")
	}
}

func TestHealthRejectsUnsafeLabelsAndOversizedSnapshot(t *testing.T) {
	for _, health := range []Health{
		{PausedReason: "budget\npaused_reason: injected"},
		{LastErrorKind: strings.Repeat("x", 65)},
	} {
		path := filepath.Join(t.TempDir(), "health.json")
		if err := WriteHealth(path, health); err == nil {
			t.Fatalf("invalid health accepted: %#v", health)
		}
	}
	path := filepath.Join(t.TempDir(), "health.json")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", maxHealthSnapshotBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadHealth(path); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("oversized snapshot error = %v", err)
	}
}
