package gitpub

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func ctxWithIdentity() context.Context { return context.Background() }

func TestCommitUsesFixedIdentityAndPostID(t *testing.T) {
	root := filepath.Join(t.TempDir(), "public")
	key := "-C " + root + " -c user.name=themachinewrites-bot -c user.email=bot@themachinewrites.com -c commit.gpgsign=false commit --no-gpg-sign -m publish:post-one"
	runner := &fakeGitRunner{results: map[string]result{key: {stdout: "committed\n"}}}
	repo := &Repository{Root: root, Runner: runner}
	sha, err := repo.Commit(ctxWithIdentity(), "post-one")
	if err != nil {
		t.Fatal(err)
	}
	if sha != "committed" {
		t.Fatalf("sha = %q", sha)
	}
	args := strings.Join(runner.calls[0], " ")
	for _, required := range []string{"-c user.name=", "-c user.email=", "commit", "--no-gpg-sign", "-m", "publish:post-one"} {
		if !strings.Contains(args, required) {
			t.Errorf("commit args missing %q: %s", required, args)
		}
	}
}

func TestCommitRejectsInvalidPostID(t *testing.T) {
	repo := &Repository{Root: filepath.Join(t.TempDir(), "public"), Runner: &fakeGitRunner{}}
	if _, err := repo.Commit(ctxWithIdentity(), "post one\ninjection"); err == nil {
		t.Fatal("invalid post ID accepted")
	}
}
