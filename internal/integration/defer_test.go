package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/daemon"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/gitpub"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/publication"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/sources"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/telemetry"
	"gopkg.in/yaml.v3"
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

type changingFixtureFetcher struct{ cycle int }

func (f *changingFixtureFetcher) Fetch(_ context.Context, _ *url.URL, _ sources.Cursor) (sources.FetchResult, error) {
	f.cycle++
	guid := fmt.Sprintf("defer-cycle-%d", f.cycle)
	body := fmt.Sprintf(`<rss version="2.0"><channel><item><guid>%s</guid><title>Fixture %d</title><link>https://example.org/%s</link><pubDate>Sun, 18 Oct 2026 20:00:00 GMT</pubDate><description>Successful fixture evidence for cycle %d.</description></item></channel></rss>`, guid, f.cycle, guid, f.cycle)
	return sources.FetchResult{Body: []byte(body)}, nil
}

func TestFakeCodexDefersAcrossSuccessfulFetchCyclesWithoutPublication(t *testing.T) {
	topology := createHarness(t)
	ctx := context.Background()
	at := time.Date(2026, 10, 18, 20, 1, 0, 0, time.UTC)
	public := topology.Public
	remoteHeadBefore := strings.TrimSpace(git(t, topology.PublicRemote, "rev-parse", "refs/heads/main"))
	postsBefore := git(t, topology.PublicRemote, "ls-tree", "-r", "--name-only", remoteHeadBefore, "--", "posts")
	latestBefore := git(t, topology.PublicRemote, "show", remoteHeadBefore+":feed/latest.json")

	fakeCodex := filepath.Join(t.TempDir(), "fake-codex")
	build := exec.Command("go", "build", "-o", fakeCodex, "./testdata/fake-codex")
	build.Dir = filepath.Join("..", "..", "internal", "integration")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fake Codex: %v: %s", err, output)
	}
	deferEngine := filepath.Join(t.TempDir(), "fake-codex-defer")
	wrapper := "#!/bin/sh\nexec " + shellQuote(fakeCodex) + " --mode defer \"$@\"\n"
	if err := os.WriteFile(deferEngine, []byte(wrapper), 0o700); err != nil {
		t.Fatal(err)
	}
	codex, err := engine.NewBoundedCodexEngine(engine.FactoryConfig{
		Executable: deferEngine, Model: "fixture-model", Timeout: 5 * time.Second,
		SandboxRoot: filepath.Join(t.TempDir(), "codex-sandbox"),
	})
	if err != nil {
		t.Fatal(err)
	}
	runtimeStore, err := store.Open(ctx, filepath.Join(topology.Root, "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeStore.Close()
	repository, err := gitpub.OpenRepository(ctx, gitpub.RepositoryConfig{
		Root: public, Branch: "main", Remote: "origin", ExpectedRemote: topology.PublicRemote,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	sourceURL := &sources.URL{}
	if err := yaml.Unmarshal([]byte("https://example.org/feed.xml"), sourceURL); err != nil {
		t.Fatal(err)
	}
	fetcher := &changingFixtureFetcher{}
	healthPath := filepath.Join(topology.Root, "runtime", "health.json")
	runtime := daemon.Runtime{
		Sources: sources.Configuration{Sources: []sources.Source{{ID: "fixture", Name: "Fixture", Category: "technology", Enabled: true, RequestInterval: time.Minute, URL: sourceURL}}},
		Fetcher: fetcher, Store: runtimeStore, Engine: codex, PublicRoot: public, HealthPath: healthPath,
		Publisher: repository, Outbox: runtimeStore, ModelName: "fixture-model", MaxCandidates: 10,
		MaxOutputTokens: 1000, Now: func() time.Time { return at },
	}
	for cycle := 0; cycle < 3; cycle++ {
		if errs := runtime.Poll(ctx); len(errs) > 0 {
			t.Fatalf("cycle %d source poll: %v", cycle+1, errs)
		}
		if err := runtime.DecideAndPublish(ctx); err != nil {
			t.Fatalf("cycle %d fake Codex decision: %v", cycle+1, err)
		}
		at = at.Add(time.Minute)
	}
	if fetcher.cycle != 3 {
		t.Fatalf("successful source fetches = %d, want 3", fetcher.cycle)
	}
	var deferred, published int
	if err := runtimeStore.DB().QueryRowContext(ctx, `SELECT count(*) FROM observed_items WHERE decision_state='deferred'`).Scan(&deferred); err != nil {
		t.Fatal(err)
	}
	if err := runtimeStore.DB().QueryRowContext(ctx, `SELECT count(*) FROM publication_outbox WHERE state='published'`).Scan(&published); err != nil {
		t.Fatal(err)
	}
	if deferred != 3 || published != 0 {
		t.Fatalf("deferred candidates=%d published outbox entries=%d, want 3 and 0", deferred, published)
	}
	remoteHeadAfter := strings.TrimSpace(git(t, topology.PublicRemote, "rev-parse", "refs/heads/main"))
	postsAfter := git(t, topology.PublicRemote, "ls-tree", "-r", "--name-only", remoteHeadAfter, "--", "posts")
	latestAfter := git(t, topology.PublicRemote, "show", remoteHeadAfter+":feed/latest.json")
	if remoteHeadAfter != remoteHeadBefore || postsAfter != postsBefore || latestAfter != latestBefore {
		t.Fatalf("defer changed public remote: head %s -> %s, posts %q -> %q, latest changed=%t", remoteHeadBefore, remoteHeadAfter, postsBefore, postsAfter, latestBefore != latestAfter)
	}
	health, err := telemetry.ReadHealth(healthPath)
	if err != nil {
		t.Fatal(err)
	}
	if health.LastPoll == nil || health.LastModelCall == nil || health.PausedReason != "insufficient_evidence" || health.LastConfirmedPush != nil {
		t.Fatalf("health after process-backed defers = %#v", health)
	}
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
