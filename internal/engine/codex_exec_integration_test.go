package engine

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestFakeCodexRunnerCanEmitDeferWithoutRemoteCalls(t *testing.T) {
	goBinary, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go executable unavailable")
	}
	executable, err := filepath.Abs("testdata/fake-codex/main.go")
	if err != nil {
		t.Fatal(err)
	}
	exec := CodexExec{
		Executable: goBinary, Sandbox: "read-only", Timeout: 10 * time.Second,
		MaxOutputBytes: 64 * 1024, TempDir: t.TempDir(), Workspace: t.TempDir(),
		Runner: &commandProcessRunner{prefix: []string{"run", executable}},
	}
	output, err := exec.Run(context.Background(), Input{Instructions: "instructions", Bundle: `{"items":[]}`})
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := ParseProposal(output.Stdout)
	if err != nil || proposal.Action != ActionDefer {
		t.Fatalf("fake defer = %#v, %v", proposal, err)
	}
}
