package engine

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Exercise the schema given to Codex and the independent Go validator with
// the same complete wire response, including required-but-unused fields.
func TestNonpublishStructuredOutputContract(t *testing.T) {
	var document any
	if err := json.Unmarshal(proposalSchema, &document); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	const schemaURL = "https://themachinewrites.com/schemas/proposal.v1.schema.json"
	if err := compiler.AddResource(schemaURL, document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []Action{ActionDefer, ActionInvestigate} {
		t.Run(string(action), func(t *testing.T) {
			response := map[string]any{
				"action": string(action), "reason": "More evidence is needed before publication.",
				"title": nil, "type": nil, "body": nil, "sources": nil,
				"topics": nil, "related_ids": nil, "requested_ids": nil,
				"correction_of": nil, "confidence_rationale": nil,
				"editorial_mode": nil, "primary_topic": nil, "factuality": nil,
				"correction_error": nil, "corrected_information": nil, "prediction": nil,
			}
			if action == ActionInvestigate {
				response["requested_ids"] = []any{"01ARZ3NDEKTSV4RRFFQ69G5FAV"}
			}
			if err := schema.Validate(response); err != nil {
				t.Fatalf("schema rejects valid nonpublication response: %v", err)
			}
			raw, err := json.Marshal(response)
			if err != nil {
				t.Fatal(err)
			}
			proposal, err := ParseProposal(raw)
			if err != nil || proposal.Action != action || proposal.Title != "" || proposal.Body != "" || len(proposal.Sources) != 0 {
				t.Fatalf("ParseProposal() = %#v, %v", proposal, err)
			}
			if action == ActionInvestigate && (len(proposal.RequestedIDs) != 1 || proposal.RequestedIDs[0] != "01ARZ3NDEKTSV4RRFFQ69G5FAV") {
				t.Fatalf("investigation lost requested IDs: %#v", proposal.RequestedIDs)
			}
			// Schema compatibility must not weaken the independent rejection of
			// contradictory content, even when each field has a valid shape.
			for field, value := range map[string]any{
				"title":   "An unwanted draft title",
				"body":    "An unwanted draft body.",
				"sources": []any{map[string]any{"title": "Source", "url": "https://example.org/source", "retrieved_at": "2026-10-21T00:00:00Z"}},
			} {
				t.Run(field, func(t *testing.T) {
					response[field] = value
					defer func() { response[field] = nil }()
					raw, err := json.Marshal(response)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := ParseProposal(raw); !errors.Is(err, ErrInvalidProposal) {
						t.Fatalf("nonpublication with %s: error = %v, want ErrInvalidProposal", field, err)
					}
				})
			}
		})
	}
}
