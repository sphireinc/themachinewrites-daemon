package publication

import (
	"strings"
	"testing"
	"time"
)

func TestRenderCanonicalPostGolden(t *testing.T) {
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	proposal := validProposal()
	proposal.RelatedIDs = []string{"01M589K0G00000000000000001"}
	id, err := GenerateID("candidate", at)
	if err != nil {
		t.Fatal(err)
	}
	post, err := Render(proposal, RenderInput{
		ID: id, PublishedAt: at, ModelIdentifier: "test-model", PromptVersion: "sha256:prompt", IndependentSources: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `---
schema_version: 1
id: ` + id + `
title: "A careful observation"
type: observation
published_at: 2026-10-18T20:00:00Z
topics: [technology]
sources:
  - title: "Primary source"
    url: https://example.org/source
    retrieved_at: 2026-10-18T20:00:00Z
related_ids: [01M589K0G00000000000000001]
correction_of: null
model_identifier: "test-model"
prompt_version: "sha256:prompt"
---

The cited evidence supports a qualified conclusion.
`
	if string(post.Markdown) != want {
		t.Fatalf("Markdown mismatch\nwant:\n%s\ngot:\n%s", want, post.Markdown)
	}
	if post.Path != "posts/2026/10/18/"+id+".md" {
		t.Fatalf("path = %q", post.Path)
	}
}

func TestRenderRejectsInvalidProvenance(t *testing.T) {
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	id, err := GenerateID("candidate", at)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Render(validProposal(), RenderInput{ID: id, PublishedAt: at, ModelIdentifier: "", PromptVersion: "sha256:prompt", IndependentSources: 1})
	if err == nil || !strings.Contains(err.Error(), "model identifier") {
		t.Fatalf("Render() error = %v", err)
	}
}
