package engine

import "context"

type Engine interface {
	Run(context.Context, Input) (Proposal, error)
}

type Input struct {
	Instructions          string
	Bundle                string
	PromptVersion         string
	EditorialMode         string
	EditorialInstructions string
}

type Action string

const (
	ActionPublish     Action = "publish"
	ActionDefer       Action = "defer"
	ActionInvestigate Action = "investigate"
)

type Proposal struct {
	Action               Action             `json:"action"`
	Reason               string             `json:"reason"`
	EditorialMode        string             `json:"editorial_mode,omitempty"`
	PrimaryTopic         string             `json:"primary_topic,omitempty"`
	Factuality           string             `json:"factuality,omitempty"`
	Title                string             `json:"title,omitempty"`
	Type                 string             `json:"type,omitempty"`
	Body                 string             `json:"body,omitempty"`
	Sources              []Source           `json:"sources,omitempty"`
	Topics               []string           `json:"topics,omitempty"`
	RelatedIDs           []string           `json:"related_ids,omitempty"`
	RequestedIDs         []string           `json:"requested_ids,omitempty"`
	CorrectionOf         string             `json:"correction_of,omitempty"`
	CorrectionError      string             `json:"correction_error,omitempty"`
	CorrectedInformation string             `json:"corrected_information,omitempty"`
	Prediction           *Prediction        `json:"prediction,omitempty"`
	ConfidenceRationale  string             `json:"confidence_rationale,omitempty"`
	PredictionReviews    []PredictionReview `json:"prediction_reviews,omitempty"`
}

type PredictionReview struct {
	PredictionID string   `json:"prediction_id"`
	Result       string   `json:"result"`
	Assessment   string   `json:"assessment"`
	Sources      []Source `json:"sources"`
}

type Prediction struct {
	Outcome                string   `json:"outcome"`
	Reasoning              string   `json:"reasoning"`
	Horizon                string   `json:"horizon"`
	Uncertainty            string   `json:"uncertainty"`
	InvalidationConditions []string `json:"invalidation_conditions"`
}

type Source struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	RetrievedAt string `json:"retrieved_at"`
}
