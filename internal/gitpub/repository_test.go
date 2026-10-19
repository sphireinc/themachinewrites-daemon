package gitpub

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeGitRunner struct {
	results map[string]result
	calls   [][]string
}

type result struct {
	stdout, stderr string
	err            error
}

func (f *fakeGitRunner) Run(_ context.Context, _ string, args, _ []string, _ []byte) result {
	f.calls = append(f.calls, append([]string(nil), args...))
	return f.results[strings.Join(args, " ")]
}

func TestRepositoryRejectsOutsidePathBranchAndRemote(t *testing.T) {
	root := t.TempDir()
	root, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &fakeGitRunner{results: map[string]result{
		"-C " + root + " rev-parse --show-toplevel":   {stdout: root + "\n"},
		"-C " + root + " rev-parse --abbrev-ref HEAD": {stdout: "main\n"},
		"-C " + root + " remote get-url origin":       {stdout: "https://github.com/example/public.git\n"},
	}}
	repo, err := OpenRepository(context.Background(), RepositoryConfig{Root: root, Branch: "main", Remote: "origin", ExpectedRemote: "https://github.com/example/public.git"}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if repo.Root != root {
		t.Fatalf("root = %s", repo.Root)
	}

	if _, err := OpenRepository(context.Background(), RepositoryConfig{Root: filepath.Join(root, "child"), Branch: "main", Remote: "origin", ExpectedRemote: "https://github.com/example/public.git"}, runner); err == nil {
		t.Fatal("outside worktree accepted")
	}
	runner.results["-C "+root+" rev-parse --abbrev-ref HEAD"] = result{stdout: "develop\n"}
	if _, err := OpenRepository(context.Background(), RepositoryConfig{Root: root, Branch: "main", Remote: "origin", ExpectedRemote: "https://github.com/example/public.git"}, runner); err == nil {
		t.Fatal("wrong branch accepted")
	}
	runner.results["-C "+root+" rev-parse --abbrev-ref HEAD"] = result{stdout: "main\n"}
	runner.results["-C "+root+" remote get-url origin"] = result{stdout: "https://github.com/other/repo.git\n"}
	if _, err := OpenRepository(context.Background(), RepositoryConfig{Root: root, Branch: "main", Remote: "origin", ExpectedRemote: "https://github.com/example/public.git"}, runner); err == nil {
		t.Fatal("wrong remote accepted")
	}
}

func TestRepositorySafePath(t *testing.T) {
	repo := &Repository{Root: "/private/tmp/public"}
	for _, unsafe := range []string{"../secret", "/etc/passwd", ".env", "runtime.sqlite", "code/main.go"} {
		if _, err := repo.SafePath(unsafe); err == nil {
			t.Errorf("unsafe path accepted: %s", unsafe)
		}
	}
	for _, safe := range []string{"posts/2026/10/18/01M589K0G00000000000000000.md", "feed/latest.json", "schemas/post.v1.schema.json"} {
		if _, err := repo.SafePath(safe); err != nil {
			t.Errorf("safe path %s: %v", safe, err)
		}
	}
}

var _ = errors.Is
