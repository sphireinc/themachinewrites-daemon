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
	if err := ValidateProposal(proposal); err != nil {
		return RenderedPost{}, err
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
	builder.WriteString("sources:\n")
	for _, source := range proposal.Sources {
		fmt.Fprintf(&builder, "  - title: %s\n    url: %s\n    retrieved_at: %s\n",
			yamlScalar(source.Title), source.URL, source.RetrievedAt)
	}
	fmt.Fprintf(&builder, "related_ids: [%s]\n", yamlList(proposal.RelatedIDs))
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
