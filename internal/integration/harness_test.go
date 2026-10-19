package integration

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/gitpub"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/observations"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/publication"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/sources"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

type harnessTopology struct {
	Root         string `json:"root"`
	Private      string `json:"private"`
	Public       string `json:"public"`
	Daemon       string `json:"daemon"`
	PublicRemote string `json:"public_remote"`
	PublicHead   string `json:"public_head"`
	RemoteHead   string `json:"remote_head"`
}

func TestBundledHarnessFixtureExists(t *testing.T) {
	if _, err := os.Stat(filepath.Join("..", "..", "tests", "harness.py")); err != nil {
		t.Fatalf("bundled integration harness is unavailable: %v", err)
	}
}

func createHarness(t *testing.T) harnessTopology {
	t.Helper()
	root := filepath.Join(t.TempDir(), "topology")
	harness := filepath.Join("..", "..", "..", "_plan", "tests", "harness.py")
	if _, err := os.Stat(harness); err != nil {
		harness = filepath.Join("..", "..", "tests", "harness.py")
	}
	command := exec.Command("python3", harness, "--root", root)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("create integration harness: %v: %s", err, output)
	}
	var topology harnessTopology
	if err := json.Unmarshal(output, &topology); err != nil {
		t.Fatalf("decode harness topology %q: %v", output, err)
	}
	return topology
}

func TestRSSFixtureToRemotePostAndWebsiteFeed(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 unavailable")
	}
	topology := createHarness(t)
	ctx := context.Background()
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)

	rss := `<rss version="2.0"><channel><item><guid>fixture-one</guid><title>A fixture observation</title><link>https://example.org/observation</link><pubDate>Sun, 18 Oct 2026 20:00:00 GMT</pubDate><description>A bounded supported observation.</description></item></channel></rss>`
	source := sources.Source{ID: "fixture", Name: "Fixture", Category: "technology", Enabled: true, RequestInterval: time.Minute}
	feedURL := mustURL(t, "https://example.org/feed.xml")
	source.URL = feedURL
	items, err := sources.ParseRSS([]byte(rss), source, at.Add(time.Minute))
	if err != nil || len(items) != 1 {
		t.Fatalf("ParseRSS() = %d, %v", len(items), err)
	}

	runtime, err := store.Open(ctx, filepath.Join(topology.Root, "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	item := items[0]
	inserted, err := observations.Ingest(ctx, runtime, observations.Candidate{
		URL: item.URL, Headline: item.Headline, SourceID: item.SourceID, SourceName: item.SourceName,
		Category: item.Category, PublishedAt: item.PublishedAt, Excerpt: item.Excerpt, RetrievedAt: item.RetrievedAt,
	})
	if err != nil || !inserted {
		t.Fatalf("Ingest() = %t, %v", inserted, err)
	}
	duplicate, err := observations.Ingest(ctx, runtime, observations.Candidate{
		URL: item.URL, Headline: item.Headline, SourceID: item.SourceID, SourceName: item.SourceName,
		Category: item.Category, PublishedAt: item.PublishedAt, Excerpt: item.Excerpt, RetrievedAt: item.RetrievedAt,
	})
	if err != nil || duplicate {
		t.Fatalf("duplicate Ingest() = %t, %v", duplicate, err)
	}

	id, err := publication.GenerateID("fixture-e2e", at)
	if err != nil {
		t.Fatal(err)
	}
	proposal := engine.Proposal{
		Action: engine.ActionPublish, Reason: "The fixture supports a bounded observation.",
		Title: "A fixture observation", Type: "observation",
		Body:    "The cited fixture contains a factual observation. This body adds a bounded interpretation without inventing evidence.",
		Sources: []engine.Source{{Title: item.SourceName, URL: item.URL, RetrievedAt: item.RetrievedAt.Format(time.RFC3339Nano)}},
		Topics:  []string{"technology"}, RelatedIDs: []string{}, ConfidenceRationale: "One deterministic fixture source supports the bounded observation.",
	}
	rendered, err := publication.Render(proposal, publication.RenderInput{
		ID: id, PublishedAt: at, ModelIdentifier: "fixture-model", PromptVersion: "fixture-prompt-v1", IndependentSources: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := publication.WriteRenderedPost(topology.Public, rendered); err != nil {
		t.Fatal(err)
	}
	corpus, err := publication.LoadCorpus(topology.Public)
	if err != nil || len(corpus.Posts) != 1 || corpus.Posts[0].ID != id {
		t.Fatalf("LoadCorpus() = %#v, %v", corpus, err)
	}
	feedPaths, err := publication.MaterializeFeeds(topology.Public, corpus.Posts, at.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{rendered.Path}
	for path := range feedPaths {
		paths = append(paths, path)
	}

	repo, err := gitpub.OpenRepository(ctx, gitpub.RepositoryConfig{Root: topology.Public, Branch: "main", Remote: "origin", ExpectedRemote: topology.PublicRemote}, nil)
	if err != nil {
		t.Fatal(err)
	}
	outbox := &memoryOutbox{}
	result, err := gitpub.Publish(ctx, repo, outbox, gitpub.PublishRequest{PostID: id, Paths: paths, MaxPushAttempts: 2})
	if err != nil {
		t.Fatal(err)
	}
	if result.CommitSHA == "" || outbox.commitSHA != result.CommitSHA {
		t.Fatalf("publish result=%#v outbox=%#v", result, outbox)
	}

	latestPath := filepath.Join(topology.Root, "remote-latest.json")
	remoteHead := git(t, topology.PublicRemote, "rev-parse", "refs/heads/main")
	latest := git(t, topology.PublicRemote, "show", strings.TrimSpace(remoteHead)+":feed/latest.json")
	if err := os.WriteFile(latestPath, []byte(latest), 0o600); err != nil {
		t.Fatal(err)
	}
	var siteCard struct {
		Title string `json:"title"`
		ID    string `json:"id"`
	}
	var latestDocument struct {
		SchemaVersion int               `json:"schema_version"`
		Posts         []json.RawMessage `json:"posts"`
	}
	if err := json.Unmarshal([]byte(latest), &latestDocument); err != nil || latestDocument.SchemaVersion != 1 || len(latestDocument.Posts) != 1 {
		t.Fatalf("remote latest = %s, %v", latest, err)
	}
	if err := json.Unmarshal(latestDocument.Posts[0], &siteCard); err != nil || siteCard.ID != id || siteCard.Title != proposal.Title {
		t.Fatalf("site card = %#v, %v", siteCard, err)
	}
	postMarkdown := git(t, topology.PublicRemote, "show", strings.TrimSpace(remoteHead)+":"+rendered.Path)
	if !strings.Contains(postMarkdown, proposal.Body) || !strings.Contains(postMarkdown, "prompt_version:") {
		t.Fatalf("canonical post from remote is incomplete")
	}
	tree := git(t, topology.PublicRemote, "ls-tree", "-r", "--name-only", strings.TrimSpace(remoteHead))
	if strings.Count(tree, "posts/") != 1 {
		t.Fatalf("remote tree does not contain exactly one post:\n%s", tree)
	}
	if exportRoot := os.Getenv("TMW_E2E_ARTIFACT_DIR"); exportRoot != "" {
		if err := os.MkdirAll(filepath.Join(exportRoot, "feed"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(exportRoot, "post-path.txt"), []byte(rendered.Path), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(exportRoot, "feed", "latest.json"), []byte(latest), 0o600); err != nil {
			t.Fatal(err)
		}
		postExport := filepath.Join(exportRoot, filepath.FromSlash(rendered.Path))
		if err := os.MkdirAll(filepath.Dir(postExport), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(postExport, []byte(postMarkdown), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

type memoryOutbox struct {
	commitSHA string
}

func (o *memoryOutbox) SavePending(context.Context, string) error { return nil }
func (o *memoryOutbox) Acknowledge(_ context.Context, _ string, sha string) error {
	o.commitSHA = sha
	return nil
}

func git(t *testing.T, repository string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repository}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(output)
}

func mustURL(t *testing.T, raw string) *sources.URL {
	t.Helper()
	configuration := "sources:\n  - id: fixture\n    url: " + raw + "\n    enabled: true\n    request_interval: 1m\n    name: Fixture\n"
	path := filepath.Join(t.TempDir(), "sources.yaml")
	if err := os.WriteFile(path, []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := sources.LoadConfiguration(path)
	if err != nil {
		t.Fatal(err)
	}
	return loaded.Sources[0].URL
}
