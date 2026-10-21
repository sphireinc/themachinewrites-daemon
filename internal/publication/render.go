package publication

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
)

type RenderInput struct {
	ID                 string
	PublishedAt        time.Time
	CorrectionOf       string
	EditorialMode      string
	PrimaryTopic       string
	ModelIdentifier    string
	PromptVersion      string
	IndependentSources int
}

type RenderedPost struct {
	ID       string
	Path     string
	Markdown []byte
}

func Render(proposal engine.Proposal, input RenderInput) (RenderedPost, error) {
	if proposal.EditorialMode == "" {
		proposal.EditorialMode = input.EditorialMode
		if proposal.EditorialMode == "" {
			proposal.EditorialMode = "sources"
		}
	}
	if proposal.PrimaryTopic == "" {
		proposal.PrimaryTopic = input.PrimaryTopic
		if proposal.PrimaryTopic == "" && len(proposal.Topics) > 0 {
			proposal.PrimaryTopic = proposal.Topics[0]
		}
		if proposal.PrimaryTopic == "" {
			proposal.Topics = []string{"general"}
			proposal.PrimaryTopic = "general"
		}
	}
	if proposal.Factuality == "" {
		if len(proposal.Sources) > 0 {
			proposal.Factuality = "source_supported"
		} else {
			proposal.Factuality = "nonfactual_reflection"
		}
	}
	if err := ValidateProposal(proposal); err != nil {
		return RenderedPost{}, err
	}
	if proposal.EditorialMode == "" || proposal.PrimaryTopic == "" {
		return RenderedPost{}, errors.New("editorial mode and primary topic are required")
	}
	if input.EditorialMode != "" && input.EditorialMode != proposal.EditorialMode {
		return RenderedPost{}, errors.New("render editorial mode does not match proposal")
	}
	if input.PrimaryTopic != "" && input.PrimaryTopic != proposal.PrimaryTopic {
		return RenderedPost{}, errors.New("render primary topic does not match proposal")
	}
	if err := ValidateCitations(proposal); err != nil {
		return RenderedPost{}, err
	}
	if err := ValidateMarkdown(proposal.Body); err != nil {
		return RenderedPost{}, err
	}
	if input.IndependentSources < 0 {
		return RenderedPost{}, errors.New("independent source count must be explicit")
	}
	if err := ValidateEvidenceQuality(proposal, input.IndependentSources); err != nil {
		return RenderedPost{}, err
	}
	if strings.TrimSpace(input.ModelIdentifier) == "" {
		return RenderedPost{}, errors.New("model identifier is required")
	}
	if strings.TrimSpace(input.PromptVersion) == "" {
		return RenderedPost{}, errors.New("prompt version is required")
	}
	if input.CorrectionOf != "" && !validULID(input.CorrectionOf) {
		return RenderedPost{}, errors.New("correction target must be a ULID")
	}
	for _, id := range proposal.RelatedIDs {
		if !validULID(id) {
			return RenderedPost{}, fmt.Errorf("related ID %q is invalid", id)
		}
	}
	for _, topic := range proposal.Topics {
		if !validTopic(topic) {
			return RenderedPost{}, fmt.Errorf("invalid topic %q", topic)
		}
	}
	path, err := CanonicalPath(input.ID, input.PublishedAt)
	if err != nil {
		return RenderedPost{}, err
	}
	var builder strings.Builder
	builder.WriteString("---\n")
	fmt.Fprintf(&builder, "schema_version: 1\nid: %s\ntitle: %s\ntype: %s\npublished_at: %s\n",
		input.ID, yamlScalar(proposal.Title), proposal.Type, input.PublishedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&builder, "topics: [%s]\n", yamlList(proposal.Topics))
	fmt.Fprintf(&builder, "editorial_mode: %s\nprimary_topic: %s\n", proposal.EditorialMode, proposal.PrimaryTopic)
	fmt.Fprintf(&builder, "factuality: %s\n", proposal.Factuality)
	if len(proposal.Sources) == 0 {
		builder.WriteString("sources: []\n")
	}
	for index, source := range proposal.Sources {
		if index == 0 {
			builder.WriteString("sources:\n")
		}
		fmt.Fprintf(&builder, "  - title: %s\n    url: %s\n    retrieved_at: %s\n",
			yamlScalar(source.Title), source.URL, source.RetrievedAt)
	}
	fmt.Fprintf(&builder, "related_ids: [%s]\n", yamlList(proposal.RelatedIDs))
	if proposal.Prediction != nil {
		predictionJSON, _ := json.Marshal(proposal.Prediction)
		fmt.Fprintf(&builder, "prediction: %s\n", string(predictionJSON))
	} else {
		builder.WriteString("prediction: null\n")
	}
	if proposal.CorrectionError != "" {
		fmt.Fprintf(&builder, "correction_error: %s\ncorrected_information: %s\n", yamlScalar(proposal.CorrectionError), yamlScalar(proposal.CorrectedInformation))
	} else {
		builder.WriteString("correction_error: null\ncorrected_information: null\n")
	}
	if input.CorrectionOf == "" {
		builder.WriteString("correction_of: null\n")
	} else {
		fmt.Fprintf(&builder, "correction_of: %s\n", input.CorrectionOf)
	}
	fmt.Fprintf(&builder, "model_identifier: %s\nprompt_version: %s\n---\n\n%s\n",
		yamlScalar(input.ModelIdentifier), yamlScalar(input.PromptVersion), strings.TrimSpace(proposal.Body))
	markdown := []byte(builder.String())
	if err := ScanRenderedPost(markdown); err != nil {
		return RenderedPost{}, err
	}
	return RenderedPost{ID: input.ID, Path: path, Markdown: markdown}, nil
}

func yamlList(values []string) string {
	return strings.Join(values, ", ")
}

func yamlScalar(value string) string {
	value = strings.TrimSpace(value)
	quoted, _ := json.Marshal(value)
	return string(quoted)
}

func validTopic(value string) bool {
	if len(value) < 1 || len(value) > 48 {
		return false
	}
	for index, r := range value {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (r == '-' && index > 0 && index < len(value)-1)) {
			return false
		}
	}
	return true
}
