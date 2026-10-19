package operations

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/gitpub"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/publication"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

func TestInspectDivergenceReportsAllThreeCategoriesWithoutRepair(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	public, remote := createDivergenceRepository(t, base)
	localOnly := mustPostID(t, "local-only", time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC))
	remoteOnly := mustPostID(t, "remote-only", time.Date(2026, 10, 18, 20, 1, 0, 0, time.UTC))
	shared := mustPostID(t, "shared", time.Date(2026, 10, 18, 20, 2, 0, 0, time.UTC))
	writeDivergencePost(t, public, shared, "Shared publication", time.Date(2026, 10, 18, 20, 2, 0, 0, time.UTC))
	writeDivergencePost(t, public, remoteOnly, "Remote-only publication", time.Date(2026, 10, 18, 20, 1, 0, 0, time.UTC))
	runTestGit(t, public, "add", ".")
	runTestGit(t, public, "commit", "-m", "remote posts")
	runTestGit(t, public, "push", "origin", "HEAD:refs/heads/main")
	runTestGit(t, public, "fetch", "origin")

	runtime, err := store.Open(ctx, filepath.Join(base, "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	at := "2026-10-18T20:03:00Z"
	if _, err := runtime.DB().ExecContext(ctx, `
		INSERT INTO observed_items(url_canonical_hash, source_id, discovered_at, fingerprint, last_seen_at, url, headline, published_at, decision_state)
		VALUES ('hash-pending','fixture',?,?,?,'https://example.org/pending','Pending candidate',?,'pending')`,
		at, "fingerprint-pending", at, at); err != nil {
		t.Fatal(err)
	}
	for _, postID := range []string{shared, localOnly} {
		if err := runtime.RestorePublicationAcknowledgement(ctx, postID, strings.Repeat("a", 40)); err != nil {
			t.Fatal(err)
		}
	}
	repo := openTestRepository(t, public, remote)
	beforeHead := strings.TrimSpace(runTestGit(t, public, "rev-parse", "HEAD"))
	beforeStatus := strings.TrimSpace(runTestGit(t, public, "status", "--porcelain"))
	report, err := InspectDivergence(ctx, runtime, repo)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report.PendingUnpublished, []string{"fingerprint-pending"}) {
		t.Fatalf("pending = %#v", report.PendingUnpublished)
	}
	if !reflect.DeepEqual(report.LocalOnlyPostIDs, []string{localOnly}) {
		t.Fatalf("local-only = %#v", report.LocalOnlyPostIDs)
	}
	if !reflect.DeepEqual(report.RemoteOnlyPostIDs, []string{remoteOnly}) {
		t.Fatalf("remote-only = %#v", report.RemoteOnlyPostIDs)
	}
	if !report.HasWarnings() {
		t.Fatal("discrepancies did not produce warnings")
	}
	if after := strings.TrimSpace(runTestGit(t, public, "rev-parse", "HEAD")); after != beforeHead {
		t.Fatalf("report changed HEAD: %s -> %s", beforeHead, after)
	}
	if after := strings.TrimSpace(runTestGit(t, public, "status", "--porcelain")); after != beforeStatus {
		t.Fatalf("report changed worktree: %q -> %q", beforeStatus, after)
	}
}

func TestInspectDivergenceCleanStateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	public, remote := createDivergenceRepository(t, base)
	id := mustPostID(t, "shared-clean", time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC))
	writeDivergencePost(t, public, id, "Shared clean publication", time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC))
	runTestGit(t, public, "add", ".")
	runTestGit(t, public, "commit", "-m", "clean history")
	runTestGit(t, public, "push", "origin", "HEAD:refs/heads/main")
	runTestGit(t, public, "fetch", "origin")
	runtime, err := store.Open(ctx, filepath.Join(base, "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if err := runtime.RestorePublicationAcknowledgement(ctx, id, strings.Repeat("a", 40)); err != nil {
		t.Fatal(err)
	}
	report, err := InspectDivergence(ctx, runtime, openTestRepository(t, public, remote))
	if err != nil {
		t.Fatal(err)
	}
	if report.HasWarnings() || report.String() != "pending_unpublished (0): none\nlocal_only_post_ids (0): none\nremote_only_post_ids (0): none\n" {
		t.Fatalf("clean report = %q warnings=%t", report.String(), report.HasWarnings())
	}
}

func createDivergenceRepository(t *testing.T, base string) (string, string) {
	t.Helper()
	remote := filepath.Join(base, "remote.git")
	public := filepath.Join(base, "public")
	runTestGit(t, base, "init", "--bare", remote)
	runTestGit(t, base, "-c", "init.defaultBranch=main", "init", public)
	runTestGit(t, public, "config", "user.email", "test@example.org")
	runTestGit(t, public, "config", "user.name", "Test")
	runTestGit(t, public, "remote", "add", "origin", remote)
	public, err := filepath.EvalSymlinks(public)
	if err != nil {
		t.Fatal(err)
	}
	return public, remote
}

func openTestRepository(t *testing.T, public, remote string) *gitpub.Repository {
	t.Helper()
	repo, err := gitpub.OpenRepository(context.Background(), gitpub.RepositoryConfig{
		Root: public, Branch: "main", Remote: "origin", ExpectedRemote: remote,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func mustPostID(t *testing.T, key string, at time.Time) string {
	t.Helper()
	id, err := publication.GenerateID(key, at)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func writeDivergencePost(t *testing.T, root, id, title string, at time.Time) {
	t.Helper()
	post, err := publication.Render(engine.Proposal{
		Action: engine.ActionPublish, Type: "observation", Title: title,
		Body:    "The fixture contains a complete public observation.",
		Sources: []engine.Source{{Title: "Source", URL: "https://example.org/source", RetrievedAt: at.Format(time.RFC3339)}},
	}, publication.RenderInput{ID: id, PublishedAt: at, ModelIdentifier: "fixture", PromptVersion: "p1", IndependentSources: 1})
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
}

func runTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(output)
}
