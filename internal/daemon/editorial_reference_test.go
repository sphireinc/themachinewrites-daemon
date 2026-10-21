package daemon

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
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
		Sources:    []engine.Source{{Title: "Evidence", URL: "https://example.org/evidence"}},
		Prediction: &engine.Prediction{Outcome: "Outcome", Reasoning: "Reason", Horizon: "By 2027", Uncertainty: "Uncertain", InvalidationConditions: []string{"Trend reverses"}}}}
	context := priorPublicationContext(posts, 10)
	if len(context) != 1 || context[0].EditorialMode != "predictions" || context[0].PrimaryTopic != "technology" || context[0].Prediction == nil || len(context[0].Sources) != 1 {
		t.Fatalf("prior publication context = %#v", context)
	}
	if len(context[0].Excerpt) > 1600 || !utf8.ValidString(context[0].Excerpt) {
		t.Fatalf("excerpt is unbounded or invalid UTF-8: %d bytes", len(context[0].Excerpt))
	}
}
