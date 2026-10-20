package integration

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/gitpub"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/publication"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

type failFirstAckOutbox struct {
	attempts int
	sha      string
	store    gitpub.Outbox
}

func (o *failFirstAckOutbox) SavePending(ctx context.Context, postID string) error {
	return o.store.SavePending(ctx, postID)
}
func (o *failFirstAckOutbox) Acknowledge(ctx context.Context, postID string, sha string) error {
	o.attempts++
	if o.attempts == 1 {
		return errors.New("simulated local acknowledgement crash")
	}
	if err := o.store.Acknowledge(ctx, postID, sha); err != nil {
		return err
	}
	o.sha = sha
	return nil
}

type successOutbox struct {
	attempts int
	sha      string
	store    gitpub.Outbox
}

func (o *successOutbox) SavePending(ctx context.Context, postID string) error {
	return o.store.SavePending(ctx, postID)
}
func (o *successOutbox) Acknowledge(ctx context.Context, postID string, sha string) error {
	o.attempts++
	if err := o.store.Acknowledge(ctx, postID, sha); err != nil {
		return err
	}
	o.sha = sha
	return nil
}

func TestPushSuccessThenAckCrashRecoversWithoutDuplicateCommit(t *testing.T) {
	topology := createHarness(t)
	ctx := context.Background()
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	id, err := publication.GenerateID("recovery-e2e", at)
	if err != nil {
		t.Fatal(err)
	}
	proposal := engine.Proposal{
		Action: engine.ActionPublish, Reason: "Recovery fixture.",
		Title: "A recovery fixture", Type: "observation", Body: "A durable observation survives an acknowledgement crash.",
		Sources: []engine.Source{{Title: "Fixture source", URL: "https://example.org/recovery", RetrievedAt: at.Add(-time.Minute).Format(time.RFC3339Nano)}},
		Topics:  []string{"technology"}, RelatedIDs: []string{}, ConfidenceRationale: "The fixture supplies a complete source.",
	}
	rendered, err := publication.Render(proposal, publication.RenderInput{ID: id, PublishedAt: at, ModelIdentifier: "fixture", PromptVersion: "p1", IndependentSources: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := publication.WriteRenderedPost(topology.Public, rendered); err != nil {
		t.Fatal(err)
	}
	feeds, err := publication.MaterializeFeeds(topology.Public, []publication.PostRecord{{ID: id, Title: proposal.Title, Type: proposal.Type, PublishedAt: at, Topics: proposal.Topics, Body: proposal.Body}}, at.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{rendered.Path}
	for path := range feeds {
		paths = append(paths, path)
	}
	repo, err := gitpub.OpenRepository(ctx, gitpub.RepositoryConfig{Root: topology.Public, Branch: "main", Remote: "origin", ExpectedRemote: topology.PublicRemote}, nil)
	if err != nil {
		t.Fatal(err)
	}
	runtimePath := filepath.Join(topology.Root, "runtime.sqlite")
	firstStore, err := store.Open(ctx, runtimePath)
	if err != nil {
		t.Fatal(err)
	}
	outbox := &failFirstAckOutbox{store: firstStore}
	request := gitpub.PublishRequest{PostID: id, Paths: paths, MaxPushAttempts: 2}
	if _, err := gitpub.Publish(ctx, repo, outbox, request); err == nil || !strings.Contains(err.Error(), "acknowledgement crash") {
		t.Fatalf("first publish error = %v", err)
	}
	if outbox.attempts != 1 {
		t.Fatalf("attempts after simulated crash = %d", outbox.attempts)
	}
	headAfterCrash := strings.TrimSpace(git(t, topology.PublicRemote, "rev-parse", "refs/heads/main"))
	if err := firstStore.Close(); err != nil {
		t.Fatal(err)
	}
	restartedStore, err := store.Open(ctx, runtimePath)
	if err != nil {
		t.Fatal(err)
	}
	defer restartedStore.Close()
	restartedOutbox := &successOutbox{store: restartedStore}
	result, err := gitpub.Publish(ctx, repo, restartedOutbox, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.CommitSHA != headAfterCrash || restartedOutbox.sha != headAfterCrash || restartedOutbox.attempts != 1 {
		t.Fatalf("recovery after restart result=%#v outbox=%#v remote=%s", result, restartedOutbox, headAfterCrash)
	}
	var storedState, storedSHA string
	if err := restartedStore.DB().QueryRowContext(ctx, `SELECT state, commit_sha FROM publication_outbox WHERE post_id = ?`, id).Scan(&storedState, &storedSHA); err != nil {
		t.Fatal(err)
	}
	if storedState != "published" || storedSHA != headAfterCrash {
		t.Fatalf("persisted outbox after restart = %s/%s, want published/%s", storedState, storedSHA, headAfterCrash)
	}
	if count := strings.Count(git(t, topology.PublicRemote, "log", "--format=%H", headAfterCrash, "--", filepath.ToSlash(rendered.Path)), headAfterCrash); count != 1 {
		t.Fatalf("recovery produced duplicate commit history")
	}
	tree := git(t, topology.PublicRemote, "ls-tree", "-r", "--name-only", headAfterCrash)
	if strings.Count(tree, "posts/") != 1 {
		t.Fatalf("recovery tree has duplicate or missing posts:\n%s", tree)
	}
}
