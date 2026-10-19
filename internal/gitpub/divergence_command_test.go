package gitpub

import (
	"context"
	"strings"
	"testing"
	"time"
)

type reportingGitRunner struct {
	root       string
	remoteOnly string
}

func (r *reportingGitRunner) Run(_ context.Context, _ string, args, _ []string, _ []byte) result {
	command := strings.TrimPrefix(strings.Join(args, " "), "-C "+r.root+" ")
	switch {
	case command == "fetch origin":
		return result{}
	case command == "rev-parse --verify origin/main^{commit}":
		return result{stdout: strings.Repeat("a", 40) + "\n"}
	case command == "ls-tree -r -z --name-only "+strings.Repeat("a", 40)+" -- posts":
		return result{stdout: "posts/2026/10/18/" + r.remoteOnly + ".md\x00"}
	case strings.HasPrefix(command, "show "+strings.Repeat("a", 40)+":posts/"):
		at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
		return result{stdout: "---\nschema_version: 1\nid: " + r.remoteOnly + "\ntitle: Remote fixture\ntype: observation\npublished_at: " + at.Format(time.RFC3339) + "\ntopics: []\nsources:\n  - title: Source\n    url: https://example.org/source\n    retrieved_at: " + at.Format(time.RFC3339) + "\nrelated_ids: []\ncorrection_of: null\nmodel_identifier: fixture\nprompt_version: p1\n---\n\nA canonical fixture body.\n"}
	default:
		return result{stderr: "unexpected git command: " + command}
	}
}

func TestRemotePostIDsReportsCanonicalIDsWithoutMutatingGit(t *testing.T) {
	root := t.TempDir()
	remoteOnly := "01M589K0G00000000000000002"
	runner := &reportingGitRunner{root: root, remoteOnly: remoteOnly}
	repo := &Repository{Root: root, Branch: "main", Remote: "origin", Runner: runner}
	ids, err := RemotePostIDs(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || !ids[remoteOnly] {
		t.Fatalf("remote IDs = %#v", ids)
	}
}
