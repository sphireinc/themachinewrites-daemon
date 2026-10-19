package publication

import (
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestRenderedMarkdownMatchesV1Contract(t *testing.T) {
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	id, err := GenerateID("candidate-contract", at)
	if err != nil {
		t.Fatal(err)
	}
	post, err := Render(validProposal(), RenderInput{
		ID: id, PublishedAt: at, ModelIdentifier: "test-model", PromptVersion: "sha256:prompt",
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

func TestUnsupportedSchemaVersionHasNoCoercion(t *testing.T) {
	if err := ValidateSchemaVersion(2); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("ValidateSchemaVersion(2) = %v", err)
	}
	if err := ValidateSchemaVersion(1); err != nil {
		t.Fatal(err)
	}
}
