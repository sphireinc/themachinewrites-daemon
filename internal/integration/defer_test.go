package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/gitpub"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/publication"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/telemetry"
)

type deferEngine struct{}

func (deferEngine) Run(context.Context, engine.Input) (engine.Proposal, error) {
	return engine.Proposal{Action: engine.ActionDefer, Reason: "The fixture does not warrant publication."}, nil
}

func TestDeferAcrossCyclesLeavesPublicationHistoryAndFeedUntouched(t *testing.T) {
	topology := createHarness(t)
	ctx := context.Background()
	public := topology.Public
	headBefore := git(t, public, "rev-parse", "HEAD")
	postsBefore := git(t, public, "ls-tree", "-r", "--name-only", "HEAD", "--", "posts")
	latestBefore, err := os.ReadFile(filepath.Join(public, "feed", "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	lastModelCall := time.Date(2026, 10, 18, 20, 5, 0, 0, time.UTC)

	for cycle := 0; cycle < 3; cycle++ {
		decision, err := engine.Decide(ctx, deferEngine{}, engine.Input{
			Instructions: "Decide from fixture evidence.", Bundle: `{"items":[]}`, PromptVersion: "fixture-prompt-v1",
		}, public)
		if err != nil {
			t.Fatal(err)
		}
		if decision.Proposal.Action != engine.ActionDefer || decision.ContentCandidate != nil || decision.PublicWrite {
			t.Fatalf("cycle %d defer decision = %#v", cycle, decision)
		}
		lastModelCall = lastModelCall.Add(time.Minute)
	}

	healthPath := filepath.Join(topology.Root, "runtime", "health.json")
	if err := telemetry.WriteHealth(healthPath, telemetry.Health{LastModelCall: &lastModelCall, PausedReason: "insufficient_evidence"}); err != nil {
		t.Fatal(err)
	}
	health, err := telemetry.ReadHealth(healthPath)
	if err != nil || health.LastModelCall == nil || !health.LastModelCall.Equal(lastModelCall) || health.PausedReason != "insufficient_evidence" || health.LastConfirmedPush != nil {
		t.Fatalf("health after defer = %#v, %v", health, err)
	}

	if headAfter := git(t, public, "rev-parse", "HEAD"); headAfter != headBefore {
		t.Fatalf("defer advanced HEAD: before=%s after=%s", headBefore, headAfter)
	}
	if postsAfter := git(t, public, "ls-tree", "-r", "--name-only", "HEAD", "--", "posts"); postsAfter != postsBefore {
		t.Fatalf("defer changed posts: before=%q after=%q", postsBefore, postsAfter)
	}
	latestAfter, err := os.ReadFile(filepath.Join(public, "feed", "latest.json"))
	if err != nil || string(latestAfter) != string(latestBefore) {
		t.Fatalf("defer changed latest feed: %v", err)
	}
	var latest publication.LatestFeed
	if err := json.Unmarshal(latestAfter, &latest); err != nil || len(latest.Posts) != 0 {
		t.Fatalf("latest feed after defer = %#v, %v", latest, err)
	}
	if _, err := gitpub.OpenRepository(ctx, gitpub.RepositoryConfig{Root: public, Branch: "main", Remote: "origin", ExpectedRemote: topology.PublicRemote}, nil); err != nil {
		t.Fatal(err)
	}
}
