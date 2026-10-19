package gitpub

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStageExactAllowlistedPaths(t *testing.T) {
	root := filepath.Join(t.TempDir(), "public")
	runner := &fakeGitRunner{results: map[string]result{}}
	repo := &Repository{Root: root, Runner: runner}
	paths := []string{
		"posts/2026/10/18/01M589K0G00000000000000000.md",
		"feed/latest.json",
		"feed/months/2026-10.json",
		"feed/catalog.json", "feed/stats.json", "feed/relations.json",
	}
	if err := repo.StageExact(context.Background(), paths); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("git calls = %#v", runner.calls)
	}
	got := runner.calls[0]
	for _, path := range paths {
		found := false
		for _, arg := range got {
			if arg == path {
				found = true
			}
		}
		if !found {
			t.Errorf("path not staged: %s", path)
		}
	}
	if err := repo.StageExact(context.Background(), []string{".env", "runtime.sqlite", "../escape"}); err == nil {
		t.Fatal("unsafe stage path accepted")
	}
}
