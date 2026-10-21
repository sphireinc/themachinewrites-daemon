package engine

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestProposalSchemaMatchesCodexStructuredOutputRequirements(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(proposalSchema, &schema); err != nil {
		t.Fatalf("unmarshal proposal schema: %v", err)
	}
	if schema["type"] != "object" {
		t.Fatalf("root schema type = %v, want object", schema["type"])
	}
	if schema["additionalProperties"] != false {
		t.Fatal("root schema must reject additional properties")
	}
	if keyword := unsupportedStructuredOutputKeyword(schema); keyword != "" {
		t.Fatalf("proposal schema contains unsupported structured-output keyword %q", keyword)
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("root schema has no properties object")
	}
	requiredValues, ok := schema["required"].([]any)
	if !ok {
		t.Fatal("root schema has no required list")
	}
	required := make(map[string]bool, len(requiredValues))
	for _, value := range requiredValues {
		name, ok := value.(string)
		if !ok {
			t.Fatalf("required entry %v is not a string", value)
		}
		required[name] = true
	}
	if len(required) != len(properties) {
		t.Fatalf("root schema requires %d of %d properties", len(required), len(properties))
	}
	for name := range properties {
		if !required[name] {
			t.Errorf("root property %q is not required", name)
		}
	}

	for _, name := range []string{"title", "type", "body", "sources", "topics", "related_ids", "requested_ids", "correction_of", "confidence_rationale"} {
		property, ok := properties[name].(map[string]any)
		if !ok {
			t.Fatalf("property %q is not an object", name)
		}
		if !hasNullableType(property["type"]) {
			t.Errorf("optional property %q must accept null", name)
		}
	}
	if _, ok := properties["prediction_reviews"].(map[string]any); !ok || !required["prediction_reviews"] {
		t.Fatal("prediction_reviews must be a required structured proposal field")
	}
}

func unsupportedStructuredOutputKeyword(value any) string {
	unsupported := map[string]bool{
		"allOf": true, "not": true, "dependentRequired": true, "dependentSchemas": true,
		"if": true, "then": true, "else": true, "uniqueItems": true,
	}
	var visit func(any) string
	visit = func(current any) string {
		switch value := current.(type) {
		case map[string]any:
			for key, child := range value {
				if unsupported[key] {
					return key
				}
				if found := visit(child); found != "" {
					return found
				}
			}
		case []any:
			for _, child := range value {
				if found := visit(child); found != "" {
					return found
				}
			}
		}
		return ""
	}
	return visit(value)
}

func hasNullableType(value any) bool {
	types, ok := value.([]any)
	if !ok {
		return false
	}
	seenString, seenNull := false, false
	for _, item := range types {
		switch item {
		case "string", "array":
			seenString = true
		case "null":
			seenNull = true
		}
	}
	return seenString && seenNull
}

func TestParseProposalRejectsDuplicateStructuredArrayValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Proposal)
	}{
		{name: "topics", mutate: func(p *Proposal) { p.Topics = []string{"technology", "technology"} }},
		{name: "related ids", mutate: func(p *Proposal) { p.RelatedIDs = []string{stringOfZeros(26), stringOfZeros(26)} }},
		{name: "requested ids", mutate: func(p *Proposal) { p.RequestedIDs = []string{stringOfZeros(26), stringOfZeros(26)} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			proposal := Proposal{
				Action: ActionPublish, Reason: "Evidence supports a careful publication.",
				Title: "A careful observation", Type: "observation", Body: "A concise, source-grounded observation.",
				Sources: []Source{{Title: "Primary source", URL: "https://example.org/source", RetrievedAt: "2026-10-20T00:00:00Z"}},
				Topics:  []string{"technology"}, ConfidenceRationale: "A primary source supports the claim.",
			}
			test.mutate(&proposal)
			raw, err := json.Marshal(proposal)
			if err != nil {
				t.Fatalf("marshal proposal: %v", err)
			}
			if _, err := ParseProposal(raw); err == nil {
				t.Fatal("ParseProposal accepted duplicate values")
			}
		})
	}
}

func stringOfZeros(count int) string { return fmt.Sprintf("%0*d", count, 0) }
