package publication

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCorrectionTargetMustExistDifferAndBeEarlier(t *testing.T) {
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	target, err := GenerateID("target", at)
	if err != nil {
		t.Fatal(err)
	}
	current, err := GenerateID("current", at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	posts := []PostRecord{
		{ID: target, Title: "Original", Type: "prediction", PublishedAt: at, Body: "original"},
		{ID: current, Title: "Correction", Type: "correction", PublishedAt: at.Add(time.Hour), Body: "correction"},
	}
	if err := ValidateCorrection(posts, current, target); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCorrection(posts, current, "01M589K0G00000000000000000"); err == nil {
		t.Fatal("unknown target accepted")
	}
	if err := ValidateCorrection(posts, target, target); err == nil {
		t.Fatal("self correction accepted")
	}
	if err := ValidateCorrection(posts, target, current); err == nil {
		t.Fatal("future target accepted")
	}
}

func TestCorrectionWorkflowKeepsOriginalBytesAndBuildsBacklinks(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	targetID, _ := GenerateID("original", at)
	correctionID, _ := GenerateID("correction", at.Add(time.Hour))
	original, _ := Render(validProposal(), RenderInput{ID: targetID, PublishedAt: at, ModelIdentifier: "test", PromptVersion: "p1"})
	correctionProposal := validProposal()
	correctionProposal.Type = "correction"
	correctionProposal.Title = "A correction to the earlier view"
	correction, _ := Render(correctionProposal, RenderInput{ID: correctionID, PublishedAt: at.Add(time.Hour), CorrectionOf: targetID, ModelIdentifier: "test", PromptVersion: "p1"})
	for _, post := range []RenderedPost{original, correction} {
		path := filepath.Join(root, filepath.FromSlash(post.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, post.Markdown, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(original.Path)))
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := LoadCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCorrectionByTime(corpus.Posts[1].PublishedAt, corpus.Posts[0].PublishedAt); err != nil {
		t.Fatal(err)
	}
	relations := BuildRelations(corpus.Posts, at.Add(2*time.Hour))
	if len(relations.Relations[targetID]) != 1 || relations.Relations[targetID][0] != correctionID {
		t.Fatalf("relations=%#v", relations.Relations)
	}
	after, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(original.Path)))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("original bytes changed")
	}
}
