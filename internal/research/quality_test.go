package research

import (
	"testing"
	"time"
)

func TestOfflineQualityFixturesFlagUnsupportedAndDuplicateEvidence(t *testing.T) {
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	wire := "The finance ministry announced a new budget after an official briefing in the capital city."
	papers := []Evidence{{SourceID: "wire", Text: wire}, {SourceID: "paper-a", Text: wire}, {SourceID: "paper-b", Text: wire}}
	if IndependentEvidenceCount(papers) != 1 {
		t.Fatal("syndicated fixture was treated as independent evidence")
	}
	if _, err := NewClaim("Fabricated claim", []SourceReference{{SourceID: "source", URL: "https://example.invalid/source", RetrievedAt: at}}); err == nil {
		t.Fatal("known invalid fixture URL accepted")
	}
	groups := GroupSyndicatedEvidence([]Evidence{{SourceID: "wire", Text: wire}, {SourceID: "paper", Text: wire}})
	if len(groups) != 1 || len(groups[0].SourceIDs) != 2 {
		t.Fatalf("groups = %#v", groups)
	}
}
