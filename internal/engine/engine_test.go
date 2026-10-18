package engine

import (
	"context"
	"testing"
)

type fakeEngine struct{ proposal Proposal }

func (f fakeEngine) Run(context.Context, Input) (Proposal, error) {
	return f.proposal, nil
}

func TestEngineInterfaceSupportsReplaceableStrategy(t *testing.T) {
	var runner Engine = fakeEngine{proposal: Proposal{Action: ActionDefer, Reason: "No meaningful contribution."}}
	got, err := runner.Run(context.Background(), Input{Instructions: "Evaluate evidence.", Bundle: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != ActionDefer {
		t.Fatalf("Action = %q, want defer", got.Action)
	}
}
