package publication

import (
	"testing"
	"time"
)

func TestIDIsStableForCandidateAndMapsToUTCDatePath(t *testing.T) {
	at := time.Date(2026, 10, 18, 23, 59, 0, 0, time.UTC)
	first, err := GenerateID("candidate-123", at)
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateID("candidate-123", at)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) != 26 {
		t.Fatalf("IDs = %q/%q", first, second)
	}
	path, err := CanonicalPath(first, at)
	if err != nil {
		t.Fatal(err)
	}
	if path != "posts/2026/10/18/"+first+".md" {
		t.Fatalf("path = %q", path)
	}
	other, err := GenerateID("candidate-456", at)
	if err != nil {
		t.Fatal(err)
	}
	if other == first {
		t.Fatal("distinct candidates shared ID")
	}
}

func TestIDRejectsInvalidCandidateAndPrecision(t *testing.T) {
	if _, err := GenerateID("", time.Now()); err == nil {
		t.Fatal("empty candidate accepted")
	}
	at := time.Date(2026, 10, 18, 20, 0, 0, 1, time.UTC)
	if _, err := GenerateID("candidate", at); err == nil {
		t.Fatal("sub-millisecond timestamp accepted")
	}
}
