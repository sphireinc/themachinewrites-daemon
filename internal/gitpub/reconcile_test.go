package gitpub

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/publication"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

type restoreRecorder struct {
	acknowledgements map[string]string
	conflict         bool
}

func (r *restoreRecorder) RestorePublicationAcknowledgement(_ context.Context, postID, commitSHA string) error {
	if r.conflict {
		return store.ErrPublicationConflict
	}
	r.acknowledgements[postID] = commitSHA
	return nil
}

func TestReconcilePublicRestoresCanonicalPostsIdempotentlyWithoutGitMutations(t *testing.T) {
	ctx := context.Background()
	public, remote := createReconcileRepository(t)
	postIDs := createPublishedFixtures(t, public)
	head := commitReconcileFixture(t, public, "seed publication history")

	database := filepath.Join(t.TempDir(), "runtime.sqlite")
	runtime, err := store.Open(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := OpenRepository(ctx, RepositoryConfig{
		Root: public, Branch: "main", Remote: "origin", ExpectedRemote: remote,
	}, commandRunner{})
	if err != nil {
		runtime.Close()
		t.Fatal(err)
	}
	for iteration := 0; iteration < 2; iteration++ {
		result, err := ReconcilePublic(ctx, repo, runtime)
		if err != nil {
			t.Fatalf("reconcile iteration %d: %v", iteration, err)
		}
		if result.CommitSHA != head || !reflect.DeepEqual(result.PostIDs, postIDs) {
			t.Fatalf("result = %#v, want sha %s and IDs %v", result, head, postIDs)
		}
	}
	if got := runGit(t, public, "rev-parse", "HEAD"); strings.TrimSpace(got) != head {
		t.Fatalf("local head changed: %q", got)
	}
	if got := runGit(t, remote, "rev-parse", "refs/heads/main"); strings.TrimSpace(got) != head {
		t.Fatalf("remote head changed: %q", got)
	}
	if got := strings.TrimSpace(runGit(t, public, "status", "--porcelain")); got != "" {
		t.Fatalf("public worktree changed: %q", got)
	}
	for _, postID := range postIDs {
		var state, commitSHA string
		if err := runtime.DB().QueryRowContext(ctx, `
			SELECT o.state, o.commit_sha
			FROM publication_outbox o WHERE o.post_id = ?`, postID).Scan(&state, &commitSHA); err != nil {
			t.Fatal(err)
		}
		if state != "published" || commitSHA != head {
			t.Fatalf("%s = %s/%s", postID, state, commitSHA)
		}
	}
	if err := runtime.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var count int
	if err := reopened.DB().QueryRow("SELECT count(*) FROM publication_outbox WHERE state = 'published'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != len(postIDs) {
		t.Fatalf("restored count = %d, want %d", count, len(postIDs))
	}
}

func TestReconcilePublicRejectsMalformedHistoryBeforeAcknowledgement(t *testing.T) {
	ctx := context.Background()
	public, remote := createReconcileRepository(t)
	createPublishedFixtures(t, public)
	path := filepath.Join(public, "posts", "2026", "10", "18", "01M589K0G00000000000000009.md")
	writeFile(t, path, "---\nschema_version: 1\nid: not-a-ulid\n---\n\nbody\n")
	commitReconcileFixture(t, public, "malformed post")
	repo, err := OpenRepository(ctx, RepositoryConfig{
		Root: public, Branch: "main", Remote: "origin", ExpectedRemote: remote,
	}, commandRunner{})
	if err != nil {
		t.Fatal(err)
	}
	recorder := &restoreRecorder{acknowledgements: map[string]string{}}
	if _, err := ReconcilePublic(ctx, repo, recorder); err == nil {
		t.Fatal("malformed history accepted")
	}
	if len(recorder.acknowledgements) != 0 {
		t.Fatalf("partial acknowledgements written: %#v", recorder.acknowledgements)
	}
}

func TestReconcilePublicPropagatesConflictingRestore(t *testing.T) {
	ctx := context.Background()
	public, remote := createReconcileRepository(t)
	createPublishedFixtures(t, public)
	commitReconcileFixture(t, public, "publication history")
	repo, err := OpenRepository(ctx, RepositoryConfig{
		Root: public, Branch: "main", Remote: "origin", ExpectedRemote: remote,
	}, commandRunner{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = ReconcilePublic(ctx, repo, &restoreRecorder{
		acknowledgements: map[string]string{}, conflict: true,
	})
	if !errors.Is(err, store.ErrPublicationConflict) {
		t.Fatalf("error = %v, want publication conflict", err)
	}
}

func TestRestorePublicationAcknowledgementRejectsDifferentCommit(t *testing.T) {
	ctx := context.Background()
	runtime, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	sha := strings.Repeat("a", 40)
	if err := runtime.RestorePublicationAcknowledgement(ctx, "post-one", sha); err != nil {
		t.Fatal(err)
	}
	if err := runtime.RestorePublicationAcknowledgement(ctx, "post-one", sha); err != nil {
		t.Fatalf("idempotent restore: %v", err)
	}
	err = runtime.RestorePublicationAcknowledgement(ctx, "post-one", strings.Repeat("b", 40))
	if !errors.Is(err, store.ErrPublicationConflict) {
		t.Fatalf("conflict error = %v", err)
	}
}

func createReconcileRepository(t *testing.T) (string, string) {
	t.Helper()
	if _, err := os.Stat("/usr/bin/git"); err != nil {
		t.Skip("git unavailable")
	}
	base := t.TempDir()
	remote := filepath.Join(base, "remote.git")
	public := filepath.Join(base, "public")
	runGit(t, base, "init", "--bare", remote)
	runGit(t, base, "-c", "init.defaultBranch=main", "init", public)
	runGit(t, public, "config", "user.email", "test@example.org")
	runGit(t, public, "config", "user.name", "Test")
	runGit(t, public, "remote", "add", "origin", remote)
	canonical, err := filepath.EvalSymlinks(public)
	if err != nil {
		t.Fatal(err)
	}
	return canonical, remote
}

func createPublishedFixtures(t *testing.T, public string) []string {
	t.Helper()
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	ids := make([]string, 0, 2)
	for index := 0; index < 2; index++ {
		id, err := publication.GenerateID("reconcile-"+string(rune('a'+index)), at.Add(time.Duration(index)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		proposal := engine.Proposal{
			Action: engine.ActionPublish, Type: "observation", Title: "Reconciled post " + string(rune('A'+index)),
			Body:    "A verified body drawn from the recorded source.",
			Sources: []engine.Source{{Title: "Source", URL: "https://example.org/source", RetrievedAt: at.Format(time.RFC3339)}},
		}
		post, err := publication.Render(proposal, publication.RenderInput{
			ID: id, PublishedAt: at.Add(time.Duration(index) * time.Minute),
			ModelIdentifier: "test-model", PromptVersion: "sha256:test", IndependentSources: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(public, filepath.FromSlash(post.Path)), string(post.Markdown))
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func commitReconcileFixture(t *testing.T, public, message string) string {
	t.Helper()
	runGit(t, public, "add", ".")
	runGit(t, public, "commit", "-m", message)
	runGit(t, public, "push", "origin", "HEAD:refs/heads/main")
	runGit(t, public, "fetch", "origin")
	return strings.TrimSpace(runGit(t, public, "rev-parse", "HEAD"))
}
