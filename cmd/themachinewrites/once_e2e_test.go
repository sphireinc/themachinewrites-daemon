package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/config"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/daemon"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/gitpub"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/sources"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/supervisor"
)

// TestOncePublishesFixtureRSSThroughFakeCodex runs the real CLI once cycle with a
// deterministic RSS fixture and fake Codex output. It uses a local bare Git remote
// and never contacts GitHub, Cloudflare, or a paid model.
func TestOncePublishesFixtureRSSThroughFakeCodex(t *testing.T) {
	for _, tool := range []string{"python3", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " unavailable")
		}
	}
	root := t.TempDir()
	topology := createOnceTopology(t, filepath.Join(root, "topology"))
	public, remote := topology.Public, topology.PublicRemote

	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	runtimeDir := filepath.Join(root, "runtime")
	codexHome := filepath.Join(root, "codex-home")
	for _, directory := range []string{runtimeDir, codexHome} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	sourcesPath := filepath.Join(root, "sources.yaml")
	sourceFile := "sources:\n  - id: fixture\n    url: https://fixture.example/feed.xml\n    enabled: true\n    request_interval: 1m\n    name: Fixture\n    category: technology\n"
	if err := os.WriteFile(sourcesPath, []byte(sourceFile), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "daemon.yaml")
	configFile := "dry_run: false\n" +
		"sources_path: " + sourcesPath + "\n" +
		"runtime_dir: " + runtimeDir + "\n" +
		"codex_executable: fixture\n" +
		"codex_timeout: 5s\n" +
		"codex_model: fixture-model\n" +
		"codex_home: " + codexHome + "\n" +
		"public_repo_path: " + public + "\n" +
		"public_branch: main\n" +
		"public_remote: https://github.com/example/public.git\n" +
		"budget:\n  daily_model_invocations: 1\n  daily_input_tokens: 100000\n  daily_output_tokens: 4000\n  max_attempts: 2\n"
	if err := os.WriteFile(configPath, []byte(configFile), 0o600); err != nil {
		t.Fatal(err)
	}

	factory := func(ctx context.Context, cfg config.Config, configured sources.Configuration) (supervisor.Cycle, func() error, error) {
		// The CLI validates a production HTTPS remote; the fixture publisher targets
		// the disposable local bare remote supplied by the harness instead.
		cfg.PublicRemote = remote
		runtime, err := store.Open(ctx, filepath.Join(cfg.RuntimeDir, "runtime.sqlite"))
		if err != nil {
			return supervisor.Cycle{}, func() error { return nil }, err
		}
		if _, err := runtime.RequeueInterruptedCandidates(ctx, at); err != nil {
			runtime.Close()
			return supervisor.Cycle{}, func() error { return nil }, err
		}
		repository, err := gitpub.OpenRepository(ctx, gitpub.RepositoryConfig{
			Root: cfg.PublicRepoPath, Branch: cfg.PublicBranch, Remote: "origin", ExpectedRemote: remote,
		}, nil)
		if err != nil {
			runtime.Close()
			return supervisor.Cycle{}, func() error { return nil }, err
		}
		runtimeCycle := daemon.Runtime{
			Sources: configured, Fetcher: fixtureFetcher{}, Store: runtime, Engine: fakePublishEngine{},
			PublicRoot: cfg.PublicRepoPath, ModelName: cfg.CodexModel, Now: func() time.Time { return at },
			Publisher: repository, Outbox: runtime, MaxCandidates: 20, MaxOutputTokens: 4000,
			Budget: supervisor.NewBudget(at, supervisor.BudgetLimits{Invocations: 1, InputTokens: 100000, OutputTokens: 4000, MaxRetries: 2}),
		}
		return supervisor.Cycle{Poll: runtimeCycle.Poll, Decide: runtimeCycle.DecideAndPublish}, runtime.Close, nil
	}

	var stdout, stderr bytes.Buffer
	if code := runWithFactory([]string{"once", "--config", configPath}, &stdout, &stderr, factory); code != 0 {
		t.Fatalf("once exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "once complete") {
		t.Fatalf("stdout=%q", stdout.String())
	}

	remoteHead := strings.TrimSpace(gitFixtureOutput(t, remote, "rev-parse", "refs/heads/main"))
	tree := gitFixtureOutput(t, remote, "ls-tree", "-r", "--name-only", remoteHead)
	if strings.Count(tree, "posts/") != 1 {
		t.Fatalf("remote tree does not contain exactly one post:\n%s", tree)
	}
	latest := gitFixtureOutput(t, remote, "show", remoteHead+":feed/latest.json")
	var latestDocument struct {
		SchemaVersion int `json:"schema_version"`
		Posts         []struct {
			Title string `json:"title"`
		} `json:"posts"`
	}
	if err := json.Unmarshal([]byte(latest), &latestDocument); err != nil {
		t.Fatalf("decode latest feed: %v", err)
	}
	if latestDocument.SchemaVersion != 1 || len(latestDocument.Posts) != 1 || latestDocument.Posts[0].Title != "A careful observation" {
		t.Fatalf("published latest feed = %s", latest)
	}
	for _, manifest := range []string{"feed/catalog.json", "feed/stats.json", "feed/relations.json"} {
		body := gitFixtureOutput(t, remote, "show", remoteHead+":"+manifest)
		var document struct {
			SchemaVersion int `json:"schema_version"`
		}
		if err := json.Unmarshal([]byte(body), &document); err != nil || document.SchemaVersion != 1 {
			t.Fatalf("%s is not canonical schema v1: %s", manifest, body)
		}
	}
}

type onceTopology struct {
	Public       string `json:"public"`
	PublicRemote string `json:"public_remote"`
}

func createOnceTopology(t *testing.T, root string) onceTopology {
	t.Helper()
	harness := filepath.Join("..", "..", "..", "_plan", "tests", "harness.py")
	if _, err := os.Stat(harness); err != nil {
		harness = filepath.Join("..", "..", "tests", "harness.py")
	}
	command := exec.Command("python3", harness, "--root", root)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("create integration harness: %v: %s", err, output)
	}
	var topology onceTopology
	if err := json.Unmarshal(output, &topology); err != nil {
		t.Fatalf("decode topology: %v: %s", err, output)
	}
	if topology.Public == "" || topology.PublicRemote == "" {
		t.Fatalf("incomplete topology: %#v", topology)
	}
	return topology
}

type fixtureFetcher struct{}

func (fixtureFetcher) Fetch(context.Context, *url.URL, sources.Cursor) (sources.FetchResult, error) {
	rss := `<rss version="2.0"><channel><item><guid>fixture-once</guid><title>A fixture observation</title><link>https://example.org/observation</link><pubDate>Sun, 18 Oct 2026 20:00:00 GMT</pubDate><description>A bounded supported observation.</description></item></channel></rss>`
	return sources.FetchResult{Body: []byte(rss), ETag: `"fixture"`}, nil
}

type fakePublishEngine struct{}

func (fakePublishEngine) Run(context.Context, engine.Input) (engine.Proposal, error) {
	return engine.Proposal{
		Action: engine.ActionPublish,
		Reason: "The fixture supports one bounded observation.",
		Title:  "A careful observation",
		Type:   "observation",
		Body:   "The fixture contains a factual observation that supports this bounded analysis.",
		Sources: []engine.Source{{
			Title: "Fixture", URL: "https://example.org/observation", RetrievedAt: "2026-10-18T20:00:00Z",
		}},
		Topics:              []string{"technology"},
		RelatedIDs:          []string{},
		ConfidenceRationale: "One deterministic fixture source supports the bounded observation.",
	}, nil
}

func gitFixtureOutput(t *testing.T, repository string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repository}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(output)
}
