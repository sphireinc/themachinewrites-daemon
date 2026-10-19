package engine

import "context"

type Engine interface {
	Run(context.Context, Input) (Proposal, error)
}

type Input struct {
	Instructions  string
	Bundle        string
	PromptVersion string
}

type Action string

const (
	ActionPublish     Action = "publish"
	ActionDefer       Action = "defer"
	ActionInvestigate Action = "investigate"
)

type Proposal struct {
	Action              Action   `json:"action"`
	Reason              string   `json:"reason"`
	Title               string   `json:"title,omitempty"`
	Type                string   `json:"type,omitempty"`
	Body                string   `json:"body,omitempty"`
	Sources             []Source `json:"sources,omitempty"`
	Topics              []string `json:"topics,omitempty"`
	RelatedIDs          []string `json:"related_ids,omitempty"`
	RequestedIDs        []string `json:"requested_ids,omitempty"`
	ConfidenceRationale string   `json:"confidence_rationale,omitempty"`
}

type Source struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	RetrievedAt string `json:"retrieved_at"`
}
