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
