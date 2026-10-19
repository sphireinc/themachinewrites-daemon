package engine

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/prompts"
)

func TestCodexEngineRequiresCurrentPromptVersion(t *testing.T) {
	bundle, err := prompts.Load()
	if err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{result: []byte(`{"action":"defer","reason":"No meaningful contribution."}`)}
	root, workspace, tempDir := codexSandboxDirs(t)
	engine := CodexEngine{Exec: CodexExec{
		Executable: "codex", Sandbox: "read-only", Timeout: time.Second,
		MaxOutputBytes: 4096, Runner: runner, SandboxRoot: root, TempDir: tempDir, Workspace: workspace,
	}}
	proposal, err := engine.Run(context.Background(), Input{
		Instructions: bundle.Text, Bundle: `{"items":[]}`, PromptVersion: bundle.Version,
	})
	if err != nil || proposal.Action != ActionDefer {
		t.Fatalf("CodexEngine.Run() = %#v, %v", proposal, err)
	}
	_, err = engine.Run(context.Background(), Input{
		Instructions: "substituted", Bundle: `{"items":[]}`, PromptVersion: "sha256:wrong",
	})
	if err == nil || !strings.Contains(err.Error(), "prompt version") {
		t.Fatalf("mismatched prompt error = %v", err)
	}
}
