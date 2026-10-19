package gitpub

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPushFastForwardOnly(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	base := t.TempDir()
	remote := filepath.Join(base, "remote.git")
	local := filepath.Join(base, "local")
	runGit(t, base, "init", "--bare", remote)
	runGit(t, base, "clone", remote, local)
	runGit(t, local, "config", "user.email", "test@example.org")
	runGit(t, local, "config", "user.name", "Test")
	writeFile(t, filepath.Join(local, "posts", "2026", "10", "18", "01M589K0G00000000000000000.md"), "post\n")
	runGit(t, local, "add", ".")
	runGit(t, local, "commit", "-m", "post")
	repo := &Repository{Root: local, Branch: "main", Remote: "origin", Runner: commandRunner{}}
	sha, err := repo.PushFastForward(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(strings.TrimSpace(sha)) != 40 && len(strings.TrimSpace(sha)) != 64 {
		t.Fatalf("sha = %q", sha)
	}
	head := runGit(t, base, "--git-dir", remote, "rev-parse", "refs/heads/main")
	if strings.TrimSpace(head) != strings.TrimSpace(sha) {
		t.Fatalf("remote head %q != %q", head, sha)
	}
}

func TestPushNeverUsesForceAfterRebase(t *testing.T) {
	root := filepath.Join(t.TempDir(), "public")
	runner := &fakeGitRunner{results: map[string]result{
		"-C " + root + " fetch origin":                                 {stdout: ""},
		"-C " + root + " rev-parse --verify origin/main":               {stdout: "remote\n"},
		"-C " + root + " rebase origin/main":                           {stdout: "rebase\n"},
		"-C " + root + " rev-parse HEAD":                               {stdout: "headsha\n"},
		"-C " + root + " push --porcelain origin HEAD:refs/heads/main": {stdout: "pushed\n"},
	}}
	repo := &Repository{Root: root, Branch: "main", Remote: "origin", Runner: runner}
	sha, err := repo.PushFastForward(context.Background(), 1)
	if err != nil || sha != "headsha" {
		t.Fatalf("PushFastForward() = %q, %v", sha, err)
	}
	args := strings.Join(runner.calls[len(runner.calls)-1], " ")
	if strings.Contains(args, "force") {
		t.Fatalf("push used a force flag: %s", args)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
