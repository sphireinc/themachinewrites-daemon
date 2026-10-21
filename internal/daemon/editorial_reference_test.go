package daemon

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/memory"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/observations"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/publication"
)

func TestEditorialReferencesResolveToCanonicalPriorPosts(t *testing.T) {
	prior := "01M589K0G00000000000000000"
	posts := []publication.PostRecord{{ID: prior}}
	for _, proposal := range []engine.Proposal{
		{EditorialMode: "follow-up", RelatedIDs: []string{prior}},
		{EditorialMode: "retrospective", RelatedIDs: []string{prior}},
		{EditorialMode: "self-review", RelatedIDs: []string{prior}},
		{EditorialMode: "corrections", Type: "correction", CorrectionOf: prior},
	} {
		if err := validateEditorialReferences(proposal, posts); err != nil {
			t.Fatalf("valid reference %#v: %v", proposal, err)
		}
		proposal.RelatedIDs = []string{"01M589K0G00000000000000001"}
		if proposal.CorrectionOf != "" {
			proposal.CorrectionOf = "01M589K0G00000000000000001"
		}
		if err := validateEditorialReferences(proposal, posts); err == nil {
			t.Fatalf("unknown reference accepted: %#v", proposal)
		}
	}
}

func TestPriorPublicationContextCarriesBoundedEvidenceAndStructuredForecast(t *testing.T) {
	posts := []publication.PostRecord{{ID: "01M589K0G00000000000000000", Title: "Forecast", Type: "prediction", EditorialMode: "predictions", PrimaryTopic: "technology",
		Topics: []string{"technology"}, PublishedAt: time.Now().UTC(), Body: strings.Repeat("界", 1000),
		Sources:    []engine.Source{{Title: "Evidence", URL: "https://example.org/evidence", RetrievedAt: "2026-10-21T14:00:00Z"}},
		Prediction: &engine.Prediction{Outcome: "Outcome", Reasoning: "Reason", Horizon: "By 2027", Uncertainty: "Uncertain", InvalidationConditions: []string{"Trend reverses"}}}}
	context := priorPublicationContext(posts, 10)
	if len(context) != 1 || context[0].EditorialMode != "predictions" || context[0].PrimaryTopic != "technology" || context[0].Prediction == nil || len(context[0].Sources) != 1 || context[0].Sources[0].RetrievedAt != "2026-10-21T14:00:00Z" {
		t.Fatalf("prior publication context = %#v", context)
	}
	if len(context[0].Excerpt) > 1600 || !utf8.ValidString(context[0].Excerpt) {
		t.Fatalf("excerpt is unbounded or invalid UTF-8: %d bytes", len(context[0].Excerpt))
	}
}

func TestPredictionReviewRequiresAnEligiblePriorForecastAndEvidence(t *testing.T) {
	postID := "01M589K0G00000000000000000"
	posts := []publication.PostRecord{{ID: postID, Type: "prediction", Prediction: &engine.Prediction{Outcome: "Outcome"}}}
	review := engine.PredictionReview{PredictionID: postID, Result: "supported", Assessment: "New evidence supports the measured outcome.",
		Sources: []engine.Source{{Title: "Evidence", URL: "https://example.org/evidence", RetrievedAt: "2026-10-21T14:00:00Z"}}}
	if err := validatePredictionReviews([]engine.PredictionReview{review}, nil, posts, nil); err == nil {
		t.Fatal("accepted review for a prediction that is not due or disputed")
	}
	if err := validatePredictionReviews([]engine.PredictionReview{review}, nil, posts, []memory.RevisitSuggestion{{PostID: postID}}); err == nil {
		t.Fatal("accepted review with evidence absent from retrieved research")
	}
	items := []observations.Candidate{{URL: "https://example.org/evidence", RetrievedAt: time.Date(2026, 10, 21, 14, 0, 0, 0, time.UTC)}}
	if err := validatePredictionReviews([]engine.PredictionReview{review}, items, posts, []memory.RevisitSuggestion{{PostID: postID}}); err != nil {
		t.Fatalf("valid prior forecast review rejected: %v", err)
	}
	review.PredictionID = "01M589K0G00000000000000001"
	if err := validatePredictionReviews([]engine.PredictionReview{review}, items, posts, []memory.RevisitSuggestion{{PostID: postID}}); err == nil {
		t.Fatal("accepted review target absent from prior publication corpus")
	}
}

func TestSparseCurrentSourcesDoNotMakeFuturePredictionsDisputed(t *testing.T) {
	now := time.Date(2026, 10, 21, 14, 0, 0, 0, time.UTC)
	post := publication.PostRecord{ID: "01M589K0G00000000000000000", Title: "Future forecast", Type: "prediction", PublishedAt: now.Add(-time.Hour),
		Prediction: &engine.Prediction{InvalidationConditions: []string{"The national policy reverses after the funding agreement is withdrawn"}}}
	for _, items := range [][]observations.Candidate{
		nil,
		{{SourceID: "one", Headline: "A single unrelated current event"}},
	} {
		if got := buildRevisitQueue([]publication.PostRecord{post}, items, now); len(got) != 0 {
			t.Fatalf("sparse unrelated sources made forecast due/disputed: %#v", got)
		}
	}
	contradictoryEvidence := []observations.Candidate{{SourceID: "report", Headline: "The national policy reverses after the funding agreement is withdrawn"}}
	if got := buildRevisitQueue([]publication.PostRecord{post}, contradictoryEvidence, now); len(got) != 1 || !got[0].Disputed {
		t.Fatalf("explicitly matched invalidation evidence did not make forecast reviewable: %#v", got)
	}
	weakEvidence := []observations.Candidate{{SourceID: "report", Headline: "Funding agreement talks continue"}}
	if got := buildRevisitQueue([]publication.PostRecord{post}, weakEvidence, now); len(got) != 0 {
		t.Fatalf("loosely related source made forecast disputed: %#v", got)
	}
}

func TestCitationMustMatchRetrievedResearchURLAndTimestamp(t *testing.T) {
	retrieved := time.Date(2026, 10, 21, 14, 0, 0, 0, time.UTC)
	items := []observations.Candidate{{URL: "https://example.org/report", RetrievedAt: retrieved}}
	proposal := engine.Proposal{Sources: []engine.Source{{Title: "Report", URL: "https://example.org/report", RetrievedAt: retrieved.Format(time.RFC3339)}}}
	if err := validateCitationOrigins(proposal, items, nil); err != nil {
		t.Fatalf("retrieved citation rejected: %v", err)
	}
	proposal.Sources[0].URL = "https://example.org/invented"
	if err := validateCitationOrigins(proposal, items, nil); err == nil {
		t.Fatal("accepted a source URL absent from retrieved research")
	}
	proposal.Sources[0].URL = "https://example.org/report"
	proposal.Sources[0].RetrievedAt = retrieved.Add(time.Minute).Format(time.RFC3339)
	if err := validateCitationOrigins(proposal, items, nil); err == nil {
		t.Fatal("accepted an invented retrieval time")
	}
}
