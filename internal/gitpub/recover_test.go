package gitpub

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecoveryAcknowledgesRemotePostWithoutDuplicateCommit(t *testing.T) {
	root := filepath.Join(t.TempDir(), "public")
	runner := &fakeGitRunner{results: map[string]result{
		"-C " + root + " log --format=%H origin/main -- posts":                        {stdout: "sha-one\n"},
		"-C " + root + " show sha-one:posts/2026/10/18/01M589K0G00000000000000000.md": {stdout: "post\n"},
	}}
	repo := &Repository{Root: root, Branch: "main", Remote: "origin", Runner: runner}
	sha, err := repo.RecoverPost(context.Background(), "01M589K0G00000000000000000")
	if err != nil || sha != "sha-one" {
		t.Fatalf("RecoverPost() = %q, %v", sha, err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("unexpected recovery command count: %#v", runner.calls)
	}
	if !strings.Contains(strings.Join(runner.calls[0], " "), "origin/main") || !strings.Contains(strings.Join(runner.calls[0], " "), "-- posts") {
		t.Fatalf("recovery did not constrain history to remote branch: %#v", runner.calls[0])
	}
}

func TestRecoveryReportsMissingPost(t *testing.T) {
	root := filepath.Join(t.TempDir(), "public")
	runner := &fakeGitRunner{results: map[string]result{}}
	repo := &Repository{Root: root, Branch: "main", Remote: "origin", Runner: runner}
	_, err := repo.RecoverPost(context.Background(), "01M589K0G00000000000000000")
	if !errors.Is(err, ErrPostNotPublished) {
		t.Fatalf("error = %v", err)
	}
}
