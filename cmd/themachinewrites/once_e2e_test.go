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
	for _, tool := range []string{"python3", "git", "go"} {
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
	fakeCodex := filepath.Join(root, "fake-codex")
	build := exec.Command("go", "build", "-o", fakeCodex, "./internal/integration/testdata/fake-codex")
	build.Dir = filepath.Join("..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fake codex: %v: %s", err, output)
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
		codexEngine, err := engine.NewBoundedCodexEngine(engine.FactoryConfig{
			Executable: fakeCodex, Model: cfg.CodexModel, Timeout: cfg.CodexTimeout,
			SandboxRoot: filepath.Join(cfg.RuntimeDir, "codex-sandbox"), CodexHome: cfg.CodexHome,
		})
		if err != nil {
			runtime.Close()
			return supervisor.Cycle{}, func() error { return nil }, err
		}
		runtimeCycle := daemon.Runtime{
			Sources: configured, Fetcher: fixtureFetcher{}, Store: runtime, Engine: codexEngine,
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
	postPath := ""
	for _, line := range strings.Split(strings.TrimSpace(tree), "\n") {
		if strings.HasPrefix(line, "posts/") {
			postPath = line
		}
	}
	if postPath == "" {
		t.Fatalf("remote tree has no post path:\n%s", tree)
	}
	latest := gitFixtureOutput(t, remote, "show", remoteHead+":feed/latest.json")
	var latestDocument struct {
		SchemaVersion int `json:"schema_version"`
		Posts         []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			PublishedAt string `json:"published_at"`
		} `json:"posts"`
	}
	if err := json.Unmarshal([]byte(latest), &latestDocument); err != nil {
		t.Fatalf("decode latest feed: %v", err)
	}
	if latestDocument.SchemaVersion != 1 || len(latestDocument.Posts) != 1 {
		t.Fatalf("published latest feed = %s", latest)
	}
	card := latestDocument.Posts[0]
	if card.Title != "A fixture observation" || card.ID == "" || card.PublishedAt == "" {
		t.Fatalf("published card is incomplete: %#v", card)
	}
	wantPath := "posts/" + card.PublishedAt[:4] + "/" + card.PublishedAt[5:7] + "/" + card.PublishedAt[8:10] + "/" + card.ID + ".md"
	if postPath != wantPath {
		t.Fatalf("post path %q does not match card ID/date; want %q", postPath, wantPath)
	}
	var catalog struct {
		SchemaVersion int `json:"schema_version"`
		Months        []struct {
			Month string `json:"month"`
			Count int    `json:"count"`
		} `json:"months"`
	}
	if err := json.Unmarshal([]byte(gitFixtureOutput(t, remote, "show", remoteHead+":feed/catalog.json")), &catalog); err != nil || catalog.SchemaVersion != 1 || len(catalog.Months) != 1 || catalog.Months[0].Count != 1 || catalog.Months[0].Month != card.PublishedAt[:7] {
		t.Fatalf("catalog does not reflect one published post: %#v, %v", catalog, err)
	}
	var stats struct {
		SchemaVersion   int    `json:"schema_version"`
		TotalPosts      int    `json:"total_posts"`
		LastPublishedAt string `json:"last_published_at"`
		ByType          struct {
			Observation int `json:"observation"`
		} `json:"by_type"`
	}
	if err := json.Unmarshal([]byte(gitFixtureOutput(t, remote, "show", remoteHead+":feed/stats.json")), &stats); err != nil || stats.SchemaVersion != 1 || stats.TotalPosts != 1 || stats.ByType.Observation != 1 || !strings.HasPrefix(stats.LastPublishedAt, card.PublishedAt) {
		t.Fatalf("stats do not reflect one observation: %#v, %v", stats, err)
	}
	var relations struct {
		SchemaVersion int                 `json:"schema_version"`
		Relations     map[string][]string `json:"relations"`
	}
	if err := json.Unmarshal([]byte(gitFixtureOutput(t, remote, "show", remoteHead+":feed/relations.json")), &relations); err != nil || relations.SchemaVersion != 1 || len(relations.Relations) != 0 {
		t.Fatalf("relations are not truthfully empty: %#v, %v", relations, err)
	}
	postMarkdown := gitFixtureOutput(t, remote, "show", remoteHead+":"+postPath)
	if !strings.Contains(postMarkdown, "schema_version: 1") || !strings.Contains(postMarkdown, "A fixture observation") {
		t.Fatalf("canonical post is incomplete: %s", postMarkdown)
	}
	if exportRoot := os.Getenv("TMW_E2E_ARTIFACT_DIR"); exportRoot != "" {
		if err := os.MkdirAll(filepath.Join(exportRoot, "feed"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(exportRoot, "post-path.txt"), []byte(postPath), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(exportRoot, "feed", "latest.json"), []byte(latest), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(exportRoot, "remote-sha.txt"), []byte(remoteHead), 0o600); err != nil {
			t.Fatal(err)
		}
		postExport := filepath.Join(exportRoot, filepath.FromSlash(postPath))
		if err := os.MkdirAll(filepath.Dir(postExport), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(postExport, []byte(postMarkdown), 0o600); err != nil {
			t.Fatal(err)
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

func gitFixtureOutput(t *testing.T, repository string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repository}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(output)
}
