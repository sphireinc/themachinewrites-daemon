package editorial

import "testing"

func TestEligibilityFiltersOnlyModesWhosePrerequisitesAreUnavailable(t *testing.T) {
	availability := Availability{RecentSources: 2, ResearchMaterial: 3}
	candidates := EligibleCandidates(availability, map[ID][]string{Sources: {"science"}, Philosophy: {"identity"}})
	got := make(map[ID]Candidate, len(candidates))
	for _, candidate := range candidates {
		got[candidate.Mode.ID] = candidate
	}
	if _, ok := got[Sources]; !ok {
		t.Fatal("sources mode was excluded despite fresh source material")
	}
	if _, ok := got[Philosophy]; !ok {
		t.Fatal("philosophy mode was excluded")
	}
	if got[FollowUp].Eligible {
		t.Fatal("follow-up mode was eligible without a follow-up target")
	}
	if got[SelfReview].Eligible {
		t.Fatal("self-review mode was eligible without prior publications")
	}
	if got[Sources].Reason == "" || got[Sources].Topics[0] != "science" {
		t.Fatalf("sources eligibility context = %#v", got[Sources])
	}
}

func TestExplicitEligibilityExplainsMissingPrerequisites(t *testing.T) {
	mode, _ := Lookup("follow-up")
	eligible, reason := CheckEligibility(mode, Availability{})
	if eligible || reason == "" || reason == "eligible" {
		t.Fatalf("eligibility = %t, reason=%q; want a clear missing-prerequisite reason", eligible, reason)
	}
}

func TestConnectionsRequiresAtLeastTwoResearchSubjects(t *testing.T) {
	mode, _ := Lookup("connections")
	eligible, reason := CheckEligibility(mode, Availability{ResearchMaterial: 1})
	if eligible || !contains(reason, "multiple research subjects") {
		t.Fatalf("connections eligibility = %t, reason=%q; want a multiple-subject prerequisite", eligible, reason)
	}
}

func TestEligibleCandidatesIncludeFreshnessAndResearchContinuitySignals(t *testing.T) {
	candidates := EligibleCandidates(Availability{
		RecentSources: 1, SourceFreshnessFactor: 0.75, ResearchMaterial: 3,
		PriorPublications: 2, FollowUpTargets: 2, OutstandingPredictions: 1, UnresolvedInvestigations: 2,
	}, nil)
	byID := make(map[ID]Candidate, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.Mode.ID] = candidate
	}
	if byID[Predictions].SignalFactor != 1.25 || byID[Predictions].FreshnessFactor != 0.75 {
		t.Fatalf("prediction factors = signal %v / freshness %v", byID[Predictions].SignalFactor, byID[Predictions].FreshnessFactor)
	}
	if byID[FollowUp].SignalFactor != 1.5 || byID[Questions].SignalFactor != 1.4 {
		t.Fatalf("follow-up/question factors = %v/%v", byID[FollowUp].SignalFactor, byID[Questions].SignalFactor)
	}
	if byID[Retrospective].SignalFactor != 1.2 {
		t.Fatalf("retrospective signal factor = %v, want 1.2", byID[Retrospective].SignalFactor)
	}
}

func contains(value, part string) bool {
	for index := 0; index+len(part) <= len(value); index++ {
		if value[index:index+len(part)] == part {
			return true
		}
	}
	return false
}
