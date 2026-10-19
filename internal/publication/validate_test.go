package publication

import (
	"strings"
	"testing"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
)

func TestValidateContentBounds(t *testing.T) {
	base := validProposal()
	if err := ValidateProposal(base); err != nil {
		t.Fatalf("valid proposal error = %v", err)
	}
	tests := []struct {
		name string
		edit func(*engine.Proposal)
	}{
		{name: "blank title", edit: func(p *engine.Proposal) { p.Title = "     " }},
		{name: "short title", edit: func(p *engine.Proposal) { p.Title = "four" }},
		{name: "oversize title", edit: func(p *engine.Proposal) { p.Title = strings.Repeat("x", 161) }},
		{name: "blank body", edit: func(p *engine.Proposal) { p.Body = " \n " }},
		{name: "oversize body", edit: func(p *engine.Proposal) { p.Body = strings.Repeat("x", 20001) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proposal := base
			tt.edit(&proposal)
			if err := ValidateProposal(proposal); err == nil {
				t.Fatal("invalid content accepted")
			}
		})
	}
}

func validProposal() engine.Proposal {
	return engine.Proposal{
		Action: engine.ActionPublish, Reason: "Evidence warrants publication.",
		Title: "A careful observation", Type: "observation",
		Body:    "The cited evidence supports a qualified conclusion.",
		Sources: []engine.Source{{Title: "Primary source", URL: "https://example.org/source", RetrievedAt: "2026-10-18T20:00:00Z"}},
		Topics:  []string{"technology"},
	}
}
