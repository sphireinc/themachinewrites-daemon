package publication

import (
	"fmt"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadCorpusReadsCanonicalPostsAndSortsDeterministically(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	for index := 0; index < 2; index++ {
		id, err := GenerateID(fmt.Sprintf("corpus-%d", index), at.Add(time.Duration(index)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		proposal := validProposal()
		proposal.Title = fmt.Sprintf("Publication number %d", index)
		proposal.RelatedIDs = nil
		if index == 1 {
			proposal.RelatedIDs = []string{mustID(t, root, at)}
		}
		post, err := Render(proposal, RenderInput{
			ID: id, PublishedAt: at.Add(time.Duration(index) * time.Minute),
			ModelIdentifier: "test-model", PromptVersion: "sha256:prompt", IndependentSources: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, filepath.FromSlash(post.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, post.Markdown, 0o600); err != nil {
			t.Fatal(err)
		}
		_ = index
	}
	corpus, err := LoadCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Posts) != 2 || !corpus.Posts[0].PublishedAt.After(corpus.Posts[1].PublishedAt) {
		t.Fatalf("posts = %#v", corpus.Posts)
	}
}

func TestLoadCorpusPreservesEditorialAndPredictionMetadata(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	id, err := GenerateID("prediction-corpus", at)
	if err != nil {
		t.Fatal(err)
	}
	proposal := validProposal()
	proposal.Type = "prediction"
	proposal.EditorialMode = "predictions"
	proposal.PrimaryTopic = "technology"
	proposal.Prediction = &engine.Prediction{Outcome: "An outcome", Reasoning: "Evidence points this way", Horizon: "By 2027", Uncertainty: "The trend may change", InvalidationConditions: []string{"The trend reverses"}}
	post, err := Render(proposal, RenderInput{ID: id, PublishedAt: at, ModelIdentifier: "test-model", PromptVersion: "sha256:prompt", IndependentSources: 1})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(post.Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, post.Markdown, 0o600); err != nil {
		t.Fatal(err)
	}
	corpus, err := LoadCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Posts) != 1 {
		t.Fatalf("post count = %d", len(corpus.Posts))
	}
	got := corpus.Posts[0]
	if got.EditorialMode != "predictions" || got.PrimaryTopic != "technology" || got.Prediction == nil || got.Prediction.Outcome != "An outcome" || got.Factuality != "source_supported" {
		t.Fatalf("canonical metadata was lost: %#v", got)
	}
}

func mustID(t *testing.T, root string, at time.Time) string {
	t.Helper()
	id, err := GenerateID("corpus-0", at)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
