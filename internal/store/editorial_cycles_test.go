package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestEditorialCycleSelectionAndOutcomeSurviveStoreRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "runtime.sqlite")
	selectedAt := time.Date(2026, 10, 21, 12, 0, 0, 123, time.UTC)
	first, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	cycle := EditorialCycle{
		ID: "cycle-1", RequestedMode: "random", SelectedMode: "philosophy",
		EligibilityReason: "always available; base_weight=1",
		EligibleModes:     []string{"philosophy", "questions"}, Topics: []string{"identity", "ethics"},
		Outcome: "pending", SelectedAt: selectedAt,
	}
	if err := first.RecordEditorialCycle(ctx, cycle); err != nil {
		t.Fatalf("RecordEditorialCycle() error = %v", err)
	}
	completedAt := selectedAt.Add(time.Minute)
	if err := first.FinishEditorialCycle(ctx, cycle.ID, "published", "01M589K0G00000000000000000", completedAt); err != nil {
		t.Fatalf("FinishEditorialCycle() error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	cycles, err := second.RecentEditorialCycles(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) != 1 || cycles[0].SelectedMode != "philosophy" || cycles[0].RequestedMode != "random" || cycles[0].Outcome != "published" || cycles[0].PublicationID != "01M589K0G00000000000000000" {
		t.Fatalf("recovered cycles = %#v", cycles)
	}
	if cycles[0].EligibilityReason != cycle.EligibilityReason || !cycles[0].SelectedAt.Equal(selectedAt) || !cycles[0].CompletedAt.Equal(completedAt) {
		t.Fatalf("recovered cycle metadata = %#v", cycles[0])
	}
	if len(cycles[0].EligibleModes) != 2 || cycles[0].EligibleModes[0] != "philosophy" || len(cycles[0].Topics) != 2 || cycles[0].Topics[1] != "ethics" {
		t.Fatalf("recovered eligibility/topic history = %#v", cycles[0])
	}
}

func TestEditorialCycleRejectsInvalidTransitions(t *testing.T) {
	ctx := context.Background()
	runtime, err := Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if err := runtime.FinishEditorialCycle(ctx, "missing", "published", "01M589K0G00000000000000000", time.Now()); err == nil {
		t.Fatal("Finishing a missing cycle succeeded")
	}
	if err := runtime.RecordEditorialCycle(ctx, EditorialCycle{ID: "", RequestedMode: "random", SelectedAt: time.Now()}); err == nil {
		t.Fatal("Recording a cycle without an ID succeeded")
	}
}

func TestEditorialCycleHistoryCanBeBoundedWithoutLosingRecentSelection(t *testing.T) {
	ctx := context.Background()
	runtime, err := Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	base := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	for index := range 4 {
		at := base.Add(time.Duration(index) * time.Minute)
		if err := runtime.RecordEditorialCycle(ctx, EditorialCycle{
			ID: string(rune('a' + index)), RequestedMode: "random", SelectedMode: "philosophy",
			EligibilityReason: "eligible", Outcome: "pending", SelectedAt: at,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := runtime.PruneEditorialCycles(ctx, 2); err != nil {
		t.Fatalf("PruneEditorialCycles() error = %v", err)
	}
	cycles, err := runtime.RecentEditorialCycles(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) != 2 || cycles[0].ID != "d" || cycles[1].ID != "c" {
		t.Fatalf("retained history = %#v, want newest IDs d,c", cycles)
	}
}
