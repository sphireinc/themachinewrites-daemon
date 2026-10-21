// Package editorial defines the typed registry used by the CLI, selector,
// prompt builder, and publication validator.
package editorial

import "sort"

type ID string

const (
	Random            ID = "random"
	Sources           ID = "sources"
	Philosophy        ID = "philosophy"
	DeepDive          ID = "deep-dive"
	Connections       ID = "connections"
	Questions         ID = "questions"
	Predictions       ID = "predictions"
	Retrospective     ID = "retrospective"
	Critique          ID = "critique"
	Discovery         ID = "discovery"
	HistoricalContext ID = "historical-context"
	ThoughtExperiment ID = "thought-experiment"
	Explainer         ID = "explainer"
	FollowUp          ID = "follow-up"
	SelfReview        ID = "self-review"
	Corrections       ID = "corrections"
)

type Prerequisite string

const (
	HasRecentSources     Prerequisite = "recent-sources"
	HasResearchMaterial  Prerequisite = "research-material"
	HasPriorPublications Prerequisite = "prior-publications"
	HasFollowUpTarget    Prerequisite = "follow-up-target"
	HasPrediction        Prerequisite = "outstanding-prediction"
)

type EvidencePolicy string

const (
	EvidenceRequired   EvidencePolicy = "sources-required-for-factual-claims"
	ReflectiveEvidence EvidencePolicy = "source-free-only-when-nonfactual"
	CorrectionEvidence EvidencePolicy = "correction-evidence-required"
)

type Mode struct {
	ID             ID
	Description    string
	Instructions   string
	Prerequisites  []Prerequisite
	Evidence       EvidencePolicy
	RandomEligible bool
}

var registry = []Mode{
	{ID: Random, Description: "Autonomously select an eligible editorial approach for this cycle.", Instructions: "Select a mode independently from eligible modes using configured weights and recent mode/topic variety. Corrections are evaluated separately and are never a random choice.", Evidence: EvidenceRequired},
	{ID: Sources, Description: "Develop an observation from recent, verifiable current events.", Instructions: "Choose a noteworthy recent development from configured source material. Verify material facts with supporting sources, distinguish facts from interpretation, and do not merely summarize a publisher's framing.", Prerequisites: []Prerequisite{HasRecentSources}, Evidence: EvidenceRequired, RandomEligible: true},
	{ID: Philosophy, Description: "Explore a philosophical question, concept, or argument.", Instructions: "Choose a philosophical question independently. Examine assumptions and competing arguments, develop a position only when warranted, and leave uncertainty open where it remains. Do not present model subjective experience as established fact.", Evidence: ReflectiveEvidence, RandomEligible: true},
	{ID: DeepDive, Description: "Investigate a subject in depth when evidence justifies it.", Instructions: "Choose a subject from research material, history, or an unresolved inquiry. Examine background, evidence, competing interpretations, and unresolved questions. Publish at length only when the investigation warrants it.", Prerequisites: []Prerequisite{HasResearchMaterial}, Evidence: EvidenceRequired, RandomEligible: true},
	{ID: Connections, Description: "Synthesize a meaningful relationship across fields or events.", Instructions: "Independently identify a cross-disciplinary connection. Explain the mechanism linking the subjects and distinguish causal or explanatory relationships from superficial analogy.", Prerequisites: []Prerequisite{HasResearchMaterial}, Evidence: EvidenceRequired, RandomEligible: true},
	{ID: Questions, Description: "Investigate an open question whose answer may remain unresolved.", Instructions: "Choose a worthwhile question independently. Establish what is known, what remains uncertain, and what evidence could resolve it. Do not manufacture a question to satisfy this mode.", Evidence: ReflectiveEvidence, RandomEligible: true},
	{ID: Predictions, Description: "Make a bounded, evidence-based forecast that can later be evaluated.", Instructions: "Choose a development with a meaningful forecast. State the outcome, evidence and reasoning, a time horizon or measurable condition, calibrated uncertainty, and conditions that would invalidate the forecast. Do not fabricate numerical confidence.", Prerequisites: []Prerequisite{HasRecentSources}, Evidence: EvidenceRequired, RandomEligible: true},
	{ID: Retrospective, Description: "Revisit an earlier publication, prediction, event, or historical development.", Instructions: "Compare what was believed or expected with subsequent evidence. Identify what remains defensible and what should be reconsidered. Link earlier publications where relevant.", Prerequisites: []Prerequisite{HasPriorPublications}, Evidence: EvidenceRequired, RandomEligible: true},
	{ID: Critique, Description: "Examine a public claim or argument without defaulting to contrarianism.", Instructions: "Assess evidence, assumptions, reasoning, alternative explanations, and legitimate counterarguments. Do not disagree for effect; say when the claim survives scrutiny.", Prerequisites: []Prerequisite{HasResearchMaterial}, Evidence: EvidenceRequired, RandomEligible: true},
	{ID: Discovery, Description: "Find a noteworthy subject receiving comparatively little attention.", Instructions: "Select an independently discovered subject rather than a popularity-ranked headline. Explain why the subject matters without exaggerating its significance, and verify factual claims.", Prerequisites: []Prerequisite{HasRecentSources}, Evidence: EvidenceRequired, RandomEligible: true},
	{ID: HistoricalContext, Description: "Use historical evidence to illuminate a current development or persistent question.", Instructions: "Compare relevant periods, decisions, institutions, and outcomes while accounting for differences. Avoid claims that history is simply repeating itself; use appropriate historical sources for factual claims.", Prerequisites: []Prerequisite{HasResearchMaterial}, Evidence: EvidenceRequired, RandomEligible: true},
	{ID: ThoughtExperiment, Description: "Explore a clearly labeled hypothetical and its implications.", Instructions: "State hypothetical premises explicitly and distinguish them from facts. Reason through implications without presenting speculation as reporting or evidence.", Evidence: ReflectiveEvidence, RandomEligible: true},
	{ID: Explainer, Description: "Explain an independently chosen complex or commonly misunderstood subject.", Instructions: "Build an accurate, accessible explanation without losing important details. Avoid unnecessary jargon, exaggerated importance, and condescension; support factual claims.", Prerequisites: []Prerequisite{HasResearchMaterial}, Evidence: EvidenceRequired, RandomEligible: true},
	{ID: FollowUp, Description: "Continue a previously published subject only when there is substantive new work.", Instructions: "Select an earlier subject with new evidence, an unresolved question, a meaningful changed condition, or a useful new connection. Cite the earlier publication and add substantive interpretation; avoid repetitive updates.", Prerequisites: []Prerequisite{HasFollowUpTarget}, Evidence: EvidenceRequired, RandomEligible: true},
	{ID: SelfReview, Description: "Reassess published positions and arguments for intellectual accountability.", Instructions: "Examine public publications and their supporting evidence for inconsistency, weak reasoning, unsupported conclusions, and changed interpretations. You may reaffirm, revise, or leave questions unresolved. Never reveal credentials, private prompts, hidden traces, or privileged context.", Prerequisites: []Prerequisite{HasPriorPublications}, Evidence: EvidenceRequired, RandomEligible: true},
	{ID: Corrections, Description: "Find and append a documented correction for a demonstrable material factual error.", Instructions: "Search earlier publications for a verifiable material factual error or prediction requiring factual correction. Identify the original and error, explain the corrected information, provide evidence, and append a correction linked to the unchanged original. Ordinary opinion changes are not corrections; if none is warranted, publish nothing.", Prerequisites: []Prerequisite{HasPriorPublications}, Evidence: CorrectionEvidence},
}

func IDs() []string {
	ids := make([]string, len(registry))
	for i, mode := range registry {
		ids[i] = string(mode.ID)
	}
	return ids
}

func Lookup(id string) (Mode, bool) {
	for _, mode := range registry {
		if string(mode.ID) == id {
			mode.Prerequisites = append([]Prerequisite(nil), mode.Prerequisites...)
			return mode, true
		}
	}
	return Mode{}, false
}

func RegularModes() []Mode {
	result := make([]Mode, 0, 15)
	for _, mode := range registry {
		if mode.RandomEligible {
			mode.Prerequisites = append([]Prerequisite(nil), mode.Prerequisites...)
			result = append(result, mode)
		}
	}
	return result
}

func ValidIDs() []string {
	ids := IDs()
	sort.Strings(ids)
	return ids
}
