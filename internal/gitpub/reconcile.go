package gitpub

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/publication"
)

type PublicationAcknowledgementRestorer interface {
	RestorePublicationAcknowledgement(context.Context, string, string) error
}

type ReconcileResult struct {
	CommitSHA string
	PostIDs   []string
}

// ReconcilePublic reconstructs local publication acknowledgements from the
// canonical posts on the configured remote branch. It reads only the remote
// tree and never stages, commits, fetches, or pushes.
func ReconcilePublic(ctx context.Context, repo *Repository, restorer PublicationAcknowledgementRestorer) (ReconcileResult, error) {
	if repo == nil || restorer == nil {
		return ReconcileResult{}, errors.New("publication repository and acknowledgement store are required")
	}
	remoteRef := repo.Remote + "/" + repo.Branch
	if _, err := repo.run(ctx, "fetch", repo.Remote); err != nil {
		return ReconcileResult{}, fmt.Errorf("refresh publication history: %w", err)
	}
	sha, err := repo.run(ctx, "rev-parse", "--verify", remoteRef+"^{commit}")
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("resolve publication commit: %w", err)
	}
	sha = strings.TrimSpace(sha)
	if !validCommitSHA(sha) {
		return ReconcileResult{}, errors.New("publication remote returned an invalid commit SHA")
	}
	tree, err := repo.run(ctx, "ls-tree", "-r", "-z", "--name-only", sha, "--", "posts")
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("list publication posts: %w", err)
	}
	ids := make([]string, 0)
	seen := map[string]bool{}
	for _, path := range strings.Split(tree, "\x00") {
		if path == "" {
			continue
		}
		record, err := readRemotePost(ctx, repo, sha, path)
		if err != nil {
			return ReconcileResult{}, err
		}
		if seen[record.ID] {
			return ReconcileResult{}, fmt.Errorf("remote publication history contains duplicate post ID %s", record.ID)
		}
		seen[record.ID] = true
		ids = append(ids, record.ID)
	}
	if len(ids) == 0 {
		return ReconcileResult{CommitSHA: sha, PostIDs: []string{}}, nil
	}
	for _, postID := range ids {
		if err := restorer.RestorePublicationAcknowledgement(ctx, postID, sha); err != nil {
			return ReconcileResult{}, fmt.Errorf("restore acknowledgement for %s: %w", postID, err)
		}
	}
	return ReconcileResult{CommitSHA: sha, PostIDs: ids}, nil
}

func readRemotePost(ctx context.Context, repo *Repository, sha, path string) (publication.PostRecord, error) {
	return readRemotePostIn(ctx, repo, sha, path, "")
}

func readRemotePostIn(ctx context.Context, repo *Repository, sha, path, root string) (publication.PostRecord, error) {
	if err := validatePostPath(path); err != nil {
		return publication.PostRecord{}, fmt.Errorf("remote post path %q: %w", path, err)
	}
	content, err := repo.run(ctx, "show", sha+":"+path)
	if err != nil {
		return publication.PostRecord{}, fmt.Errorf("read remote post %s: %w", path, err)
	}
	if root == "" {
		root, err = os.MkdirTemp("", "tmw-reconcile-")
		if err != nil {
			return publication.PostRecord{}, fmt.Errorf("create reconciliation workspace: %w", err)
		}
		defer os.RemoveAll(root)
	}
	absolute, err := (&Repository{Root: root}).SafePath(path)
	if err != nil {
		return publication.PostRecord{}, err
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
		return publication.PostRecord{}, err
	}
	if err := os.WriteFile(absolute, []byte(content), 0o600); err != nil {
		return publication.PostRecord{}, fmt.Errorf("write reconciliation fixture: %w", err)
	}
	corpus, err := publication.LoadCorpus(root)
	if err != nil {
		return publication.PostRecord{}, fmt.Errorf("validate remote post %s: %w", path, err)
	}
	if len(corpus.Posts) != 1 || corpus.Posts[0].Path != path {
		return publication.PostRecord{}, fmt.Errorf("remote post %s did not yield one canonical record", path)
	}
	return corpus.Posts[0], nil
}

func validCommitSHA(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}
