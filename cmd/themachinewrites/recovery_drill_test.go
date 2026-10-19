package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/config"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/gitpub"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/publication"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/sources"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

func TestCleanHostReplacementReconstructsAndRestartsSupervisor(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	remote := filepath.Join(base, "remote.git")
	oldHost := filepath.Join(base, "old-host")
	oldRuntime := filepath.Join(oldHost, "runtime")
	oldPublic := filepath.Join(oldHost, "public")
	newHost := filepath.Join(base, "new-host")
	newRuntime := filepath.Join(newHost, "relocated", "runtime")
	newPublic := filepath.Join(newHost, "relocated", "public")
	codexHome := filepath.Join(newHost, "protected", "codex-home")
	for _, path := range []string{oldRuntime, oldPublic, filepath.Dir(newRuntime), filepath.Dir(newPublic), codexHome} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	runRecoveryGit(t, base, "init", "--bare", "--initial-branch=main", remote)
	runRecoveryGit(t, base, "-c", "init.defaultBranch=main", "init", oldPublic)
	runRecoveryGit(t, oldPublic, "config", "user.email", "test@example.org")
	runRecoveryGit(t, oldPublic, "config", "user.name", "Test")
	runRecoveryGit(t, oldPublic, "remote", "add", "origin", remote)
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	postID, err := publication.GenerateID("host-replacement", at)
	if err != nil {
		t.Fatal(err)
	}
	post, err := publication.Render(engine.Proposal{
		Action: engine.ActionPublish, Type: "observation", Title: "Recovered publication",
		Body:    "A canonical publication reconstructed after host replacement.",
		Sources: []engine.Source{{Title: "Source", URL: "https://example.org/source", RetrievedAt: at.Format(time.RFC3339)}},
	}, publication.RenderInput{ID: postID, PublishedAt: at, ModelIdentifier: "fixture", PromptVersion: "p1", IndependentSources: 1})
	if err != nil {
		t.Fatal(err)
	}
	postPath := filepath.Join(oldPublic, filepath.FromSlash(post.Path))
	if err := os.MkdirAll(filepath.Dir(postPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(postPath, post.Markdown, 0o600); err != nil {
		t.Fatal(err)
	}
	runRecoveryGit(t, oldPublic, "add", ".")
	runRecoveryGit(t, oldPublic, "commit", "-m", "before host loss")
	runRecoveryGit(t, oldPublic, "push", "origin", "HEAD:refs/heads/main")
	remoteHead := strings.TrimSpace(runRecoveryGit(t, oldPublic, "rev-parse", "HEAD"))

	oldStore, err := store.Open(ctx, filepath.Join(oldRuntime, "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if err := oldStore.RestorePublicationAcknowledgement(ctx, postID, remoteHead); err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(base, "backup.sqlite")
	if err := oldStore.BackupTo(ctx, backupPath); err != nil {
		t.Fatal(err)
	}
	if err := oldStore.Close(); err != nil {
		t.Fatal(err)
	}
	runRecoveryGit(t, base, "clone", remote, newPublic)
	if branch := strings.TrimSpace(runRecoveryGit(t, newPublic, "branch", "--show-current")); branch != "main" {
		t.Fatalf("replacement clone branch = %q, want main", branch)
	}
	newPublic, err = filepath.EvalSymlinks(newPublic)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newRuntime, 0o700); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newRuntime, "runtime.sqlite"), backup, 0o600); err != nil {
		t.Fatal(err)
	}
	restored, err := store.Open(ctx, filepath.Join(newRuntime, "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	repository, err := gitpub.OpenRepository(ctx, gitpub.RepositoryConfig{
		Root: newPublic, Branch: "main", Remote: "origin", ExpectedRemote: remote,
	}, nil)
	if err != nil {
		restored.Close()
		t.Fatal(err)
	}
	result, err := gitpub.ReconcilePublic(ctx, repository, restored)
	if err != nil {
		restored.Close()
		t.Fatal(err)
	}
	if err := restored.Close(); err != nil {
		t.Fatal(err)
	}
	if result.CommitSHA != remoteHead || len(result.PostIDs) != 1 || result.PostIDs[0] != postID {
		t.Fatalf("reconstructed history = %#v", result)
	}
	beforeTree, err := os.ReadFile(postPath)
	if err != nil {
		t.Fatal(err)
	}
	snapshotIDs := make([]string, 0)
	if err := restoredDBQuery(ctx, filepath.Join(newRuntime, "runtime.sqlite"), func(value string) {
		snapshotIDs = append(snapshotIDs, value)
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(snapshotIDs)
	if len(snapshotIDs) != 1 || snapshotIDs[0] != postID {
		t.Fatalf("restored snapshot IDs = %v", snapshotIDs)
	}

	sourcesPath := filepath.Join(newHost, "sources.yaml")
	if err := os.WriteFile(sourcesPath, []byte("sources: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(newHost, "config.yaml")
	content := "dry_run: true\nsources_path: " + sourcesPath + "\nruntime_dir: " + newRuntime + "\ncodex_model: fixture\ncodex_home: " + codexHome + "\npublic_repo_path: " + newPublic + "\npublic_branch: main\npublic_remote: https://github.com/example/public.git\n"
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadEnvy(configPath, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	cfg.PublicRemote = remote
	replacementCredential := "replacement-token-only-in-environment"
	cfg.DryRun = true
	dryCycle, closeDryCycle, err := buildSupervisorCycleWith(ctx, cfg, sources.Configuration{}, nil, func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	if err := closeDryCycle(); err != nil {
		t.Fatal(err)
	}
	if dryCycle.Poll == nil || dryCycle.Decide == nil || !dryCycle.DryRun {
		t.Fatalf("restarted dry cycle = %#v", dryCycle)
	}
	cfg.DryRun = false
	cycle, closeCycle, err := buildSupervisorCycleWith(ctx, cfg, sources.Configuration{}, func(name string) string {
		if name == "THEMACHINEWRITES_GIT_PUBLISH_TOKEN" {
			return replacementCredential
		}
		return ""
	}, func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	if cycle.Poll == nil || cycle.Decide == nil || cycle.DryRun {
		t.Fatalf("restarted cycle = %#v", cycle)
	}
	if err := closeCycle(); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(runRecoveryGit(t, newPublic, "status", "--porcelain")); got != "" {
		t.Fatalf("new clone changed: %q", got)
	}
	afterTree, err := os.ReadFile(filepath.Join(newPublic, filepath.FromSlash(post.Path)))
	if err != nil {
		t.Fatal(err)
	}
	if string(afterTree) != string(beforeTree) {
		t.Fatal("recovery modified the canonical public post")
	}
}

func restoredDBQuery(ctx context.Context, path string, visit func(string)) error {
	restored, err := store.Open(ctx, path)
	if err != nil {
		return err
	}
	defer restored.Close()
	rows, err := restored.DB().QueryContext(ctx, "SELECT post_id FROM publication_outbox WHERE state = 'published'")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return err
		}
		visit(value)
	}
	return rows.Err()
}

func runRecoveryGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}
