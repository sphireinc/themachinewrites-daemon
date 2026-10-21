package editorial

import (
	"fmt"
	"strings"
)

type Availability struct {
	RecentSources            int
	ResearchMaterial         int
	PriorPublications        int
	FollowUpTargets          int
	OutstandingPredictions   int
	UnresolvedInvestigations int
	SourceFreshnessFactor    float64
}

func EligibleCandidates(availability Availability, topicHints map[ID][]string) []Candidate {
	result := make([]Candidate, 0, len(RegularModes()))
	for _, mode := range RegularModes() {
		eligible, reason := CheckEligibility(mode, availability)
		candidate := Candidate{Mode: mode, Eligible: eligible, Reason: reason, Topics: append([]string(nil), topicHints[mode.ID]...), SignalFactor: 1}
		for _, prerequisite := range mode.Prerequisites {
			if prerequisite == HasRecentSources {
				candidate.FreshnessFactor = availability.SourceFreshnessFactor
				break
			}
		}
		switch mode.ID {
		case Predictions:
			if availability.OutstandingPredictions > 0 {
				candidate.SignalFactor = 1 + float64(min(availability.OutstandingPredictions, 2))*0.25
				candidate.SignalReason = fmt.Sprintf("outstanding_predictions=%d", availability.OutstandingPredictions)
			}
		case Retrospective:
			if availability.OutstandingPredictions > 0 {
				candidate.SignalFactor = 1.2
				candidate.SignalReason = fmt.Sprintf("outstanding_predictions=%d", availability.OutstandingPredictions)
			}
		case FollowUp:
			if availability.FollowUpTargets > 0 {
				candidate.SignalFactor = 1 + float64(min(availability.FollowUpTargets, 2))*0.25
				candidate.SignalReason = fmt.Sprintf("follow_up_targets=%d", availability.FollowUpTargets)
			}
		case Questions:
			if availability.UnresolvedInvestigations > 0 {
				candidate.SignalFactor = 1 + float64(min(availability.UnresolvedInvestigations, 2))*0.2
				candidate.SignalReason = fmt.Sprintf("unresolved_investigations=%d", availability.UnresolvedInvestigations)
			}
		}
		result = append(result, candidate)
	}
	return result
}

func min(value, maxValue int) int {
	if value < maxValue {
		return value
	}
	return maxValue
}

func CheckEligibility(mode Mode, availability Availability) (bool, string) {
	var missing, signals []string
	for _, prerequisite := range mode.Prerequisites {
		satisfied, signal := prerequisiteStatus(prerequisite, availability)
		if !satisfied {
			missing = append(missing, signal)
		} else {
			signals = append(signals, signal)
		}
	}
	if len(missing) != 0 {
		return false, "ineligible: missing " + strings.Join(missing, "; ")
	}
	if len(signals) == 0 {
		return true, "eligible: no additional prerequisites"
	}
	return true, "eligible: " + strings.Join(signals, "; ")
}

func prerequisiteStatus(prerequisite Prerequisite, availability Availability) (bool, string) {
	switch prerequisite {
	case HasRecentSources:
		return availability.RecentSources > 0, fmt.Sprintf("recent_sources=%d", availability.RecentSources)
	case HasResearchMaterial:
		return availability.ResearchMaterial > 0, fmt.Sprintf("research_material=%d", availability.ResearchMaterial)
	case HasMultipleSubjects:
		return availability.ResearchMaterial >= 2, fmt.Sprintf("multiple research subjects available=%t (count=%d)", availability.ResearchMaterial >= 2, availability.ResearchMaterial)
	case HasPriorPublications:
		return availability.PriorPublications > 0, fmt.Sprintf("prior_publications=%d", availability.PriorPublications)
	case HasFollowUpTarget:
		return availability.FollowUpTargets > 0, fmt.Sprintf("follow_up_targets=%d", availability.FollowUpTargets)
	case HasPrediction:
		return availability.OutstandingPredictions > 0, fmt.Sprintf("outstanding_predictions=%d", availability.OutstandingPredictions)
	default:
		return false, fmt.Sprintf("unknown prerequisite %q", prerequisite)
	}
}
