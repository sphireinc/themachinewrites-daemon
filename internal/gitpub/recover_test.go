package gitpub

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestRecoveryAcknowledgesRemotePostWithoutDuplicateCommit(t *testing.T) {
	root := filepath.Join(t.TempDir(), "public")
	runner := &fakeGitRunner{results: map[string]result{
		"-C " + root + " log --all --format=%H -- posts":                              {stdout: "sha-one\n"},
		"-C " + root + " show sha-one:posts/2026/10/18/01M589K0G00000000000000000.md": {stdout: "post\n"},
	}}
	repo := &Repository{Root: root, Runner: runner}
	sha, err := repo.RecoverPost(context.Background(), "01M589K0G00000000000000000")
	if err != nil || sha != "sha-one" {
		t.Fatalf("RecoverPost() = %q, %v", sha, err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("unexpected recovery command count: %#v", runner.calls)
	}
}

func TestRecoveryReportsMissingPost(t *testing.T) {
	root := filepath.Join(t.TempDir(), "public")
	runner := &fakeGitRunner{results: map[string]result{}}
	repo := &Repository{Root: root, Runner: runner}
	_, err := repo.RecoverPost(context.Background(), "01M589K0G00000000000000000")
	if !errors.Is(err, ErrPostNotPublished) {
		t.Fatalf("error = %v", err)
	}
}
