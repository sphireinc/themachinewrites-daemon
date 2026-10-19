package research

import (
	"strings"
	"testing"
	"time"
)

func TestClaimsPreserveAccurateSourceProvenance(t *testing.T) {
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	claim, err := NewClaim("A factual claim supported by evidence.", []SourceReference{
		{SourceID: "source-a", URL: "https://example.org/a", RetrievedAt: at, Excerpt: "support"},
		{SourceID: "source-b", URL: "https://example.org/b", RetrievedAt: at.Add(time.Minute)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(claim.ID, "claim-") || len(claim.Sources) != 2 || claim.IndependentSourceCount() != 2 || !claim.Sources[0].RetrievedAt.Equal(at) {
		t.Fatalf("claim = %#v", claim)
	}
	if _, err := NewClaim("Another claim", []SourceReference{{SourceID: "source-a", URL: "https://example.org/a", RetrievedAt: at}, {SourceID: "source-a", URL: "https://example.org/a", RetrievedAt: at}}); err == nil {
		t.Fatal("duplicate source reference accepted")
	}
}

func TestClaimReferenceRejectsTokenizedQueryAndAcceptsCanonicalProvenance(t *testing.T) {
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	if _, err := NewClaim("A claim", []SourceReference{{SourceID: "source", URL: "https://example.org/a?token=secret", RetrievedAt: at}}); err == nil {
		t.Fatal("query-bearing claim reference accepted")
	}
	claim, err := NewClaim("A claim", []SourceReference{{SourceID: "source", URL: "https://example.org/a", RetrievedAt: at, Excerpt: "  a   brief excerpt "}})
	if err != nil || claim.Sources[0].Excerpt != "a brief excerpt" {
		t.Fatalf("claim = %#v, %v", claim, err)
	}
}

func TestThreeIdenticalWireRepostsFormOneIndependenceGroup(t *testing.T) {
	text := "The central bank raised interest rates by twenty five basis points on Wednesday according to an official statement."
	groups := GroupSyndicatedEvidence([]Evidence{{SourceID: "wire", Text: text}, {SourceID: "paper-a", Text: text}, {SourceID: "paper-b", Text: text}, {SourceID: "independent", Text: "Researchers reported a separate local election result with a different set of quoted officials."}})
	if len(groups) != 2 || IndependentEvidenceCount([]Evidence{{SourceID: "wire", Text: text}, {SourceID: "paper-a", Text: text}, {SourceID: "paper-b", Text: text}}) != 1 {
		t.Fatalf("groups = %#v", groups)
	}
}

func TestClaimIndependenceCollapsesWireCopyAcrossOutletIDs(t *testing.T) {
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	text := "The central bank raised interest rates by twenty five basis points on Wednesday according to an official statement."
	claim, err := NewClaim("The bank raised rates.", []SourceReference{
		{SourceID: "wire", URL: "https://wire.example/story", RetrievedAt: at, Excerpt: text},
		{SourceID: "paper-a", URL: "https://a.example/story", RetrievedAt: at, Excerpt: text},
		{SourceID: "paper-b", URL: "https://b.example/story", RetrievedAt: at, Excerpt: text},
	})
	if err != nil {
		t.Fatal(err)
	}
	if claim.IndependentSourceCount() != 1 {
		t.Fatalf("wire copy counted as %d independent sources", claim.IndependentSourceCount())
	}
}
