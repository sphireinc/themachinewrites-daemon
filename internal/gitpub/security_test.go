package gitpub

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPublicationPathsRejectTraversalSymlinkUNCAndCraftedIDs(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "public")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(filepath.Join(root, "posts", "2026", "10", "18"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	canary := filepath.Join(outside, "canary.md")
	if err := os.WriteFile(canary, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "posts", "2026", "10", "18", "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	repo := &Repository{Root: root, Runner: &fakeGitRunner{results: map[string]result{}}}
	inputs := []string{
		"../outside/canary.md",
		"posts/2026/10/18/escape/canary.md",
		`\\server\share\canary.md`,
		`C:\Windows\canary.md`,
		"posts/2026/02/30/01M589K0G00000000000000000.md",
		"posts/2026/10/18/../../../../outside/canary.md",
	}
	for _, input := range inputs {
		if _, err := repo.SafePath(filepath.ToSlash(input)); err == nil {
			t.Errorf("unsafe path accepted: %q", input)
		}
	}
	if err := repo.StageExact(context.Background(), []string{"posts/2026/10/18/escape/canary.md"}); err == nil {
		t.Fatal("stage accepted symlink escape")
	}
	got, err := os.ReadFile(canary)
	if err != nil || string(got) != "outside" {
		t.Fatalf("outside canary changed: %q, %v", got, err)
	}
}
