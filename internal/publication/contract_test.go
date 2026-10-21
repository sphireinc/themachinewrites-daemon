package publication

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

func TestRenderedMarkdownMatchesV1Contract(t *testing.T) {
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	id, err := GenerateID("candidate-contract", at)
	if err != nil {
		t.Fatal(err)
	}
	post, err := Render(validProposal(), RenderInput{
		ID: id, PublishedAt: at, ModelIdentifier: "test-model", PromptVersion: "sha256:prompt", IndependentSources: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	content := string(post.Markdown)
	if !strings.HasPrefix(content, "---\n") {
		t.Fatal("missing opening frontmatter")
	}
	end := strings.Index(content[4:], "\n---\n")
	if end < 0 {
		t.Fatal("missing closing frontmatter")
	}
	end += 4
	var metadata map[string]any
	if err := yaml.Unmarshal([]byte(content[4:end]), &metadata); err != nil {
		t.Fatalf("frontmatter parse: %v", err)
	}
	normalized, err := jsonObject(metadata)
	if err != nil {
		t.Fatalf("normalize frontmatter as JSON: %v", err)
	}
	if err := validatePinnedPostSchema(normalized); err != nil {
		t.Fatalf("rendered frontmatter violates pinned public schema: %v", err)
	}
	if metadata["schema_version"] != 1 || metadata["id"] != id || metadata["title"] != "A careful observation" ||
		metadata["type"] != "observation" || metadata["model_identifier"] != "test-model" ||
		metadata["prompt_version"] != "sha256:prompt" {
		t.Fatalf("metadata = %#v", metadata)
	}
	body := strings.TrimSpace(content[end+5:])
	if body != "The cited evidence supports a qualified conclusion." {
		t.Fatalf("body = %q", body)
	}
}

func TestNonfactualReflectionRendersWithoutSourcesAndRetainsIndependentModeAndTopic(t *testing.T) {
	proposal := validProposal()
	proposal.EditorialMode = "philosophy"
	proposal.PrimaryTopic = "identity"
	proposal.Topics = []string{"identity", "artificial-intelligence"}
	proposal.Factuality = "nonfactual_reflection"
	proposal.Sources = nil
	post, err := Render(proposal, RenderInput{ID: "01M589K0G0A46YFKY4RCSP5JX3", PublishedAt: time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC), ModelIdentifier: "test-model", PromptVersion: "sha256:prompt"})
	if err != nil {
		t.Fatal(err)
	}
	frontmatter := strings.SplitN(string(post.Markdown), "---", 3)[1]
	var metadata map[string]any
	if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err != nil {
		t.Fatal(err)
	}
	normalized, err := jsonObject(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePinnedPostSchema(normalized); err != nil {
		t.Fatalf("source-free reflective post violates contract: %v", err)
	}
	if metadata["editorial_mode"] != "philosophy" || metadata["primary_topic"] != "identity" {
		t.Fatalf("mode/topic metadata conflated: %#v", metadata)
	}
}

func jsonObject(value map[string]any) (map[string]any, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var normalized map[string]any
	return normalized, json.Unmarshal(content, &normalized)
}

func TestPinnedSchemaRejectsIntentionalIncompatibleMutation(t *testing.T) {
	schemaFile, pin := readPinnedSchema(t, "post.v1.schema.json")
	if got := sha256.Sum256(schemaFile); pin.Schemas["post.v1.schema.json"] != "sha256:"+hex.EncodeToString(got[:]) {
		t.Fatalf("pinned schema hash mismatch")
	}
	mutated := map[string]any{"schema_version": 2}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://themachinewrites.com/schemas/post.v1.schema.json", decodeSchema(t, schemaFile)); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("https://themachinewrites.com/schemas/post.v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(mutated); err == nil {
		t.Fatal("intentional schema_version 2 mutation passed pinned v1 schema")
	}
}

type contractPin struct {
	Revision string            `json:"public_repository_revision"`
	Schemas  map[string]string `json:"schemas"`
}

func readPinnedSchema(t *testing.T, name string) ([]byte, contractPin) {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", "contracts")
	pinBytes, err := os.ReadFile(filepath.Join(root, "PIN.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pin contractPin
	if err := json.Unmarshal(pinBytes, &pin); err != nil {
		t.Fatal(err)
	}
	if len(pin.Revision) != 40 || pin.Schemas[name] == "" {
		t.Fatalf("invalid contract pin: %#v", pin)
	}
	content, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return content, pin
}

func validatePinnedPostSchema(instance map[string]any) error {
	schemaFile, err := readPinnedSchemaForValidation()
	if err != nil {
		return err
	}
	schemaDocument, err := decodeSchemaForValidation(schemaFile)
	if err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource("https://themachinewrites.com/schemas/post.v1.schema.json", schemaDocument); err != nil {
		return err
	}
	schema, err := compiler.Compile("https://themachinewrites.com/schemas/post.v1.schema.json")
	if err != nil {
		return err
	}
	return schema.Validate(instance)
}

func readPinnedSchemaForValidation() ([]byte, error) {
	return os.ReadFile(filepath.Join("..", "..", "testdata", "contracts", "post.v1.schema.json"))
}

func decodeSchema(t *testing.T, content []byte) any {
	t.Helper()
	value, err := decodeSchemaForValidation(content)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func decodeSchemaForValidation(content []byte) (any, error) {
	var value any
	err := json.Unmarshal(content, &value)
	return value, err
}

func TestUnsupportedSchemaVersionHasNoCoercion(t *testing.T) {
	if err := ValidateSchemaVersion(2); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("ValidateSchemaVersion(2) = %v", err)
	}
	if err := ValidateSchemaVersion(1); err != nil {
		t.Fatal(err)
	}
}
