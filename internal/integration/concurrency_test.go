package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/daemon"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/gitpub"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/observations"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/supervisor"
)

func TestConcurrentPublisherContentionAllowsOneLeaseOnly(t *testing.T) {
	root := filepath.Join(t.TempDir(), "publishing-lease")
	start := make(chan struct{})
	type outcome struct {
		lease *supervisor.Lease
		err   error
	}
	results := make(chan outcome, 2)
	var workers sync.WaitGroup
	for _, worker := range []string{"daemon-a", "daemon-b"} {
		workers.Add(1)
		go func(workerID string) {
			defer workers.Done()
			<-start
			lease, err := supervisor.AcquireLease(root, workerID)
			results <- outcome{lease: lease, err: err}
		}(worker)
	}
	close(start)
	workers.Wait()
	close(results)

	acquired := 0
	var held *supervisor.Lease
	for result := range results {
		if result.err == nil {
			acquired++
			held = result.lease
		}
	}
	if acquired != 1 || held == nil {
		t.Fatalf("concurrent lease acquisitions = %d", acquired)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	next, err := supervisor.AcquireLease(root, "daemon-c")
	if err != nil {
		t.Fatalf("lease not available after release: %v", err)
	}
	if err := next.Release(); err != nil {
		t.Fatal(err)
	}
}

type contentionPublishEngine struct{}

func (contentionPublishEngine) Run(context.Context, engine.Input) (engine.Proposal, error) {
	return engine.Proposal{
		Action: engine.ActionPublish, Reason: "One candidate is ready.",
		Title: "A contention observation", Type: "observation",
		Body:    "A single publisher may commit this observation.",
		Sources: []engine.Source{{Title: "Fixture", URL: "https://example.org/contention", RetrievedAt: "2026-10-18T20:00:00Z"}},
		Topics:  []string{"technology"}, ConfidenceRationale: "One deterministic fixture source.",
	}, nil
}

func TestDaemonCycleHonorsPublisherLeaseAndCreatesNoCompetingCommit(t *testing.T) {
	topology := createHarness(t)
	ctx := context.Background()
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	runtime, err := store.Open(ctx, filepath.Join(topology.Root, "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if inserted, err := observations.Ingest(ctx, runtime, observations.Candidate{
		URL: "https://example.org/contention", Headline: "Contention fixture", SourceID: "fixture", SourceName: "Fixture",
		Category: "technology", PublishedAt: at, RetrievedAt: at,
	}); err != nil || !inserted {
		t.Fatalf("ingest = %t, %v", inserted, err)
	}
	repo, err := gitpub.OpenRepository(ctx, gitpub.RepositoryConfig{
		Root: topology.Public, Branch: "main", Remote: "origin", ExpectedRemote: topology.PublicRemote,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cycle := daemon.Runtime{
		Store: runtime, Engine: contentionPublishEngine{}, PublicRoot: topology.Public,
		ModelName: "fixture-model", Now: func() time.Time { return at }, Publisher: repo,
		Outbox: runtime, WorkerID: "daemon-b", MaxCandidates: 20, MaxOutputTokens: 4000,
		Budget: supervisor.NewBudget(at, supervisor.BudgetLimits{Invocations: 1, InputTokens: 100000, OutputTokens: 4000, MaxRetries: 1}),
	}
	lease, err := supervisor.AcquireLease(filepath.Join(topology.Public, ".git"), "daemon-a")
	if err != nil {
		t.Fatal(err)
	}
	headBefore := git(t, topology.PublicRemote, "rev-parse", "refs/heads/main")
	if err := cycle.DecideAndPublish(ctx); err == nil {
		t.Fatal("second daemon published while lease was held")
	}
	if headAfter := git(t, topology.PublicRemote, "rev-parse", "refs/heads/main"); headAfter != headBefore {
		t.Fatalf("blocked publisher advanced remote: %s -> %s", headBefore, headAfter)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if err := cycle.DecideAndPublish(ctx); err != nil {
		t.Fatal(err)
	}
	headAfter := strings.TrimSpace(git(t, topology.PublicRemote, "rev-parse", "refs/heads/main"))
	if headAfter == headBefore {
		t.Fatal("released lease did not allow publication")
	}
	tree := git(t, topology.PublicRemote, "ls-tree", "-r", "--name-only", headAfter)
	if strings.Count(tree, "posts/") != 1 {
		t.Fatalf("contention produced duplicate posts:\n%s", tree)
	}
	_ = os.Remove(filepath.Join(topology.Public, ".git", "publish.lock"))
}
