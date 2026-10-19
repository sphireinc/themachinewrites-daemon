package publication

import (
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"testing"
)

func TestWeaklySourcedConsequentialClaimRequiresUncertainty(t *testing.T) {
	proposal := validProposal()
	proposal.Title = "A consequential allegation"
	proposal.Body = "The official definitely caused the failure."
	if err := ValidateEvidenceQuality(proposal, 1); err == nil {
		t.Fatal("weakly sourced certainty accepted")
	}
	proposal.Body = "The official may have caused the failure; evidence is uncertain."
	if err := ValidateEvidenceQuality(proposal, 1); err != nil {
		t.Fatal(err)
	}
	if err := ValidateEvidenceQuality(proposal, 2); err != nil {
		t.Fatal(err)
	}
}

func TestSameHostDoesNotProvideIndependentCorroboration(t *testing.T) {
	proposal := validProposal()
	proposal.Title = "A consequential allegation"
	proposal.Body = "The official definitely caused the failure."
	proposal.Sources = append(proposal.Sources, engine.Source{Title: "Second from same outlet", URL: "https://example.org/other", RetrievedAt: "2026-10-18T20:01:00Z"})
	if err := ValidateEvidenceQuality(proposal, 2); err == nil {
		t.Fatal("same-host articles treated as independent corroboration")
	}
}
