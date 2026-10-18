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
