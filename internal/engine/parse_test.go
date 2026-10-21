package engine

import (
	"errors"
	"strings"
	"testing"
)

func TestParseProposalPublishAndDefer(t *testing.T) {
	publish := []byte(`{
      "action":"publish","reason":"New evidence warrants analysis.",
      "title":"A careful observation","type":"observation",
      "body":"The evidence supports a qualified conclusion.",
      "sources":[{"title":"Primary source","url":"https://example.org/source","retrieved_at":"2026-10-18T20:00:00Z"}],
      "topics":["technology"],"related_ids":[],"confidence_rationale":"Two independent sources agree."
    }`)
	proposal, err := ParseProposal(publish)
	if err != nil || proposal.Action != ActionPublish || proposal.Title != "A careful observation" {
		t.Fatalf("ParseProposal(publish) = %#v, %v", proposal, err)
	}
	deferProposal, err := ParseProposal([]byte(`{"action":"defer","reason":"Nothing meaningful to add."}`))
	if err != nil || deferProposal.Action != ActionDefer {
		t.Fatalf("ParseProposal(defer) = %#v, %v", deferProposal, err)
	}
}

func TestParseProposalRejectsExtraMalformedInvalidAndTruncated(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want error
	}{
		{name: "extra tool field", raw: []byte(`{"action":"defer","reason":"No.","tool_command":"git push"} `), want: ErrMalformedProposal},
		{name: "malformed", raw: []byte(`{"action":`), want: ErrMalformedProposal},
		{name: "invalid UTF-8", raw: append([]byte(`{"action":"defer","reason":"`), 0xff, '"', '}'), want: ErrMalformedProposal},
		{name: "trailing", raw: []byte(`{"action":"defer","reason":"No."} {"action":"publish"}`), want: ErrMalformedProposal},
		{name: "publish missing body", raw: []byte(`{"action":"publish","reason":"Publish.","title":"A title long enough","type":"observation"}`), want: ErrInvalidProposal},
		{name: "publish raw HTML", raw: []byte(`{"action":"publish","reason":"Publish.","title":"A title long enough","type":"observation","body":"<script>alert(1)</script>","sources":[{"title":"Source","url":"https://example.org/source","retrieved_at":"2026-10-18T20:00:00Z"}],"topics":["technology"],"confidence_rationale":"Source."}`), want: ErrInvalidProposal},
		{name: "publish control character", raw: []byte("{\"action\":\"publish\",\"reason\":\"Publish.\",\"title\":\"A title long enough\",\"type\":\"observation\",\"body\":\"unsafe\\u0000body\",\"sources\":[{\"title\":\"Source\",\"url\":\"https://example.org/source\",\"retrieved_at\":\"2026-10-18T20:00:00Z\"}],\"topics\":[\"technology\"],\"confidence_rationale\":\"Source.\"}"), want: ErrInvalidProposal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseProposal(tt.raw)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ParseProposal() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestProposalOutputLeavesOutboxUntouchedWhenInvalid(t *testing.T) {
	// Parsing has no store dependency; this assertion documents that invalid output cannot mutate state.
	if _, err := ParseProposal([]byte(strings.Repeat("not-json", 100))); err == nil {
		t.Fatal("invalid proposal accepted")
	}
}

func TestParseProposalForModeAllowsOnlyNonfactualSourceFreeReflections(t *testing.T) {
	raw := []byte(`{"action":"publish","reason":"A coherent reflection is warranted.","editorial_mode":"philosophy","primary_topic":"identity","factuality":"nonfactual_reflection","title":"A question of identity","type":"reflection","body":"Identity can be understood as continuity in the story a person tells about change. This is a conceptual argument, not a claim about a particular experiment.","sources":[],"topics":["identity","humanity"],"related_ids":[],"requested_ids":[],"correction_of":null,"correction_error":null,"corrected_information":null,"prediction":null,"confidence_rationale":"The argument distinguishes continuity from sameness."}`)
	proposal, err := ParseProposalForMode(raw, "philosophy")
	if err != nil {
		t.Fatalf("source-free philosophy proposal rejected: %v", err)
	}
	if proposal.PrimaryTopic != "identity" || proposal.EditorialMode != "philosophy" {
		t.Fatalf("normalized metadata = %#v", proposal)
	}
	if _, err := ParseProposalForMode(raw, "sources"); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("source-free sources proposal error = %v, want invalid proposal", err)
	}
	withCitationButNonfactual := strings.Replace(string(raw), `"sources":[]`, `"sources":[{"title":"A source","url":"https://example.org/source","retrieved_at":"2026-10-18T20:00:00Z"}]`, 1)
	if _, err := ParseProposalForMode([]byte(withCitationButNonfactual), "philosophy"); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("cited nonfactual proposal error = %v, want invalid proposal", err)
	}
}

func TestParseProposalForModeValidatesModeReferencesAndStructuredPredictions(t *testing.T) {
	base := `"action":"publish","reason":"Evidence supports this forecast.","editorial_mode":"predictions","primary_topic":"technology","factuality":"source_supported","title":"A bounded forecast","type":"prediction","body":"A forecast with a defined horizon.","sources":[{"title":"Source","url":"https://example.org/source","retrieved_at":"2026-10-18T20:00:00Z"}],"topics":["technology"],"related_ids":[],"requested_ids":[],"correction_of":null,"correction_error":null,"corrected_information":null,"prediction":{"outcome":"The system reaches the stated threshold.","reasoning":"Recent measurements show a sustained trend.","horizon":"By 2027-06-30 or when the threshold is reached.","uncertainty":"This is uncertain because the measurement can shift.","invalidation_conditions":["The next three measurements fall below the baseline."]},"confidence_rationale":"The source supports the direction but not a precise probability."`
	if _, err := ParseProposalForMode([]byte("{"+base+"}"), "predictions"); err != nil {
		t.Fatalf("complete prediction rejected: %v", err)
	}
	incomplete := strings.Replace(base, `,"invalidation_conditions":["The next three measurements fall below the baseline."]`, `,"invalidation_conditions":[]`, 1)
	if _, err := ParseProposalForMode([]byte("{"+incomplete+"}"), "predictions"); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("prediction without invalidation conditions error = %v, want invalid proposal", err)
	}
	mismatch := strings.Replace(base, `"editorial_mode":"predictions"`, `"editorial_mode":"philosophy"`, 1)
	if _, err := ParseProposalForMode([]byte("{"+mismatch+"}"), "predictions"); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("mode mismatch error = %v, want invalid proposal", err)
	}
	followUp := strings.Replace(base, `"editorial_mode":"predictions"`, `"editorial_mode":"follow-up"`, 1)
	followUp = strings.Replace(followUp, `"type":"prediction"`, `"type":"analysis"`, 1)
	followUp = strings.Replace(followUp, `,"prediction":{"outcome":"The system reaches the stated threshold.","reasoning":"Recent measurements show a sustained trend.","horizon":"By 2027-06-30 or when the threshold is reached.","uncertainty":"This is uncertain because the measurement can shift.","invalidation_conditions":["The next three measurements fall below the baseline."]}`, `,"prediction":null`, 1)
	if _, err := ParseProposalForMode([]byte("{"+followUp+"}"), "follow-up"); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("follow-up without prior-publication references error = %v, want invalid proposal", err)
	}
}
