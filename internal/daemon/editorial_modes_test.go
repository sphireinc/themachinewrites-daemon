package daemon

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/editorial"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/observations"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/publication"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

type editorialCaptureEngine struct {
	inputs []engine.Input
}

func (e *editorialCaptureEngine) Run(_ context.Context, input engine.Input) (engine.Proposal, error) {
	e.inputs = append(e.inputs, input)
	return engine.Proposal{Action: engine.ActionDefer, Reason: "No warranted publication."}, nil
}

func TestExplicitPhilosophyRunsWithoutPendingSourceCandidates(t *testing.T) {
	ctx := context.Background()
	state, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	engineStub := &editorialCaptureEngine{}
	at := time.Date(2026, 10, 21, 14, 0, 0, 0, time.UTC)
	runtime := Runtime{
		Store: state, Engine: engineStub, PublicRoot: t.TempDir(), DryRun: true,
		EditorialMode: editorial.Philosophy,
		Now:           func() time.Time { return at },
	}
	if err := runtime.DecideAndPublish(ctx); err != nil {
		t.Fatal(err)
	}
	if len(engineStub.inputs) != 1 {
		t.Fatalf("engine calls = %d, want one source-independent editorial cycle", len(engineStub.inputs))
	}
	var bundle map[string]any
	if err := json.Unmarshal([]byte(engineStub.inputs[0].Bundle), &bundle); err != nil {
		t.Fatal(err)
	}
	if bundle["editorial_mode"] != "philosophy" {
		t.Fatalf("decision bundle editorial_mode = %#v", bundle["editorial_mode"])
	}
	history, err := state.RecentEditorialCycles(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].SelectedMode != "philosophy" || history[0].Outcome != "deferred" {
		t.Fatalf("editorial cycle history = %#v", history)
	}
}

func TestRandomEditorialModeIsResolvedAndRecordedForEachCycle(t *testing.T) {
	ctx := context.Background()
	state, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	engineStub := &editorialCaptureEngine{}
	at := time.Date(2026, 10, 21, 14, 0, 0, 0, time.UTC)
	draws := 0
	runtime := Runtime{
		Store: state, Engine: engineStub, PublicRoot: t.TempDir(), DryRun: true,
		EditorialMode: editorial.Random,
		EditorialSettings: editorial.SelectionConfig{
			Weights: map[string]int{"philosophy": 1, "questions": 1}, ModeRepeatWindow: 1,
			ModeRepeatPenalty: 0,
		},
		RandomBelow: func(uint64) (uint64, error) { draws++; return 0, nil },
		Now:         func() time.Time { return at.Add(time.Duration(draws) * time.Millisecond) },
	}
	for range 2 {
		if err := runtime.DecideAndPublish(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if len(engineStub.inputs) != 2 || draws != 2 {
		t.Fatalf("engine calls/random draws = %d/%d, want 2/2", len(engineStub.inputs), draws)
	}
	var first, second map[string]any
	if err := json.Unmarshal([]byte(engineStub.inputs[0].Bundle), &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(engineStub.inputs[1].Bundle), &second); err != nil {
		t.Fatal(err)
	}
	if first["editorial_mode"] != "philosophy" || second["editorial_mode"] != "questions" {
		t.Fatalf("cycle modes = %v, %v; want philosophy then questions", first["editorial_mode"], second["editorial_mode"])
	}
}

func TestRecentSourceOnPublishedTopicMakesFollowUpModeEligible(t *testing.T) {
	posts := []publication.PostRecord{{ID: "01M589K0G00000000000000000", Topics: []string{"science"}}}
	items := []observations.Candidate{{Category: "science"}}
	runtime := Runtime{}
	availability := runtime.editorialAvailability(items, posts, nil, nil)
	if availability.FollowUpTargets != 1 {
		t.Fatalf("follow-up targets = %d, want one new-source match to a published topic", availability.FollowUpTargets)
	}
	hints := editorialTopicHints(items, posts, nil)
	if len(hints[editorial.FollowUp]) != 1 || hints[editorial.FollowUp][0] != "science" {
		t.Fatalf("follow-up topic hints = %v, want [science]", hints[editorial.FollowUp])
	}
}
