package editorial

import (
	"errors"
	"testing"
)

func TestChooseExcludesIneligibleAndZeroWeightModes(t *testing.T) {
	candidates := []Candidate{
		{Mode: Mode{ID: Sources, RandomEligible: true}, Eligible: true, Reason: "recent source material"},
		{Mode: Mode{ID: Philosophy, RandomEligible: true}, Eligible: false, Reason: "no research material"},
		{Mode: Mode{ID: Questions, RandomEligible: true}, Eligible: true, Reason: "always available"},
	}
	selection, err := Choose(candidates, SelectionConfig{Weights: map[string]int{"sources": 2, "philosophy": 9, "questions": 0}}, nil, func(max uint64) (uint64, error) {
		if max != 2_000_000 {
			t.Fatalf("random bound = %d, want 2000000", max)
		}
		return 0, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if selection.Mode == nil || selection.Mode.ID != Sources {
		t.Fatalf("selected mode = %#v, want sources", selection.Mode)
	}
	if len(selection.Eligible) != 1 || selection.Eligible[0].Mode.ID != Sources {
		t.Fatalf("eligible candidates = %#v, want only sources", selection.Eligible)
	}
}

func TestChooseAppliesModeAndTopicRepetitionPenaltiesIndependently(t *testing.T) {
	candidates := []Candidate{
		{Mode: Mode{ID: Sources, RandomEligible: true}, Eligible: true, Topics: []string{"science"}},
		{Mode: Mode{ID: Philosophy, RandomEligible: true}, Eligible: true, Topics: []string{"identity"}},
	}
	settings := SelectionConfig{
		Weights:          map[string]int{"sources": 1, "philosophy": 1},
		ModeRepeatWindow: 1, ModeRepeatPenalty: 0.5,
		TopicRepeatWindow: 1, TopicRepeatPenalty: 0.5,
	}
	// Recent mode repetition suppresses philosophy; recent topic repetition
	// independently suppresses sources. The remaining weights are equal.
	history := []RecentSelection{{Mode: Philosophy, Topics: []string{"science"}}}
	selection, err := Choose(candidates, settings, history, func(uint64) (uint64, error) { return 0, nil })
	if err != nil {
		t.Fatal(err)
	}
	if selection.Mode == nil || selection.Mode.ID != Sources {
		t.Fatalf("selected mode = %#v, want sources after independent penalties", selection.Mode)
	}
	if selection.Eligible[0].ModePenalty != 1 || selection.Eligible[0].TopicPenalty != 0.5 {
		t.Fatalf("sources penalties = mode %v, topic %v, want 1 and 0.5", selection.Eligible[0].ModePenalty, selection.Eligible[0].TopicPenalty)
	}
	if selection.Eligible[1].ModePenalty != 0.5 || selection.Eligible[1].TopicPenalty != 1 {
		t.Fatalf("philosophy penalties = mode %v, topic %v, want 0.5 and 1", selection.Eligible[1].ModePenalty, selection.Eligible[1].TopicPenalty)
	}
}

func TestChooseReportsNoEligibleWeightedModeWithoutFallback(t *testing.T) {
	candidates := []Candidate{{Mode: Mode{ID: Questions, RandomEligible: true}, Eligible: true}}
	selection, err := Choose(candidates, SelectionConfig{Weights: map[string]int{"questions": 0}}, nil, func(uint64) (uint64, error) {
		t.Fatal("RNG was called without a positive eligible weight")
		return 0, nil
	})
	if !errors.Is(err, ErrNoEligibleMode) || selection.Mode != nil {
		t.Fatalf("Choose() = (%#v, %v), want no eligible weighted mode", selection, err)
	}
}

func TestChoosePropagatesRandomSourceFailure(t *testing.T) {
	candidates := []Candidate{{Mode: Mode{ID: Sources, RandomEligible: true}, Eligible: true}}
	wantErr := errors.New("rng failed")
	_, err := Choose(candidates, SelectionConfig{Weights: map[string]int{"sources": 1}}, nil, func(uint64) (uint64, error) { return 0, wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("Choose() error = %v, want %v", err, wantErr)
	}
}

func TestChooseUsesSourceFreshnessAsASeparateWeightFactor(t *testing.T) {
	candidates := []Candidate{
		{Mode: Mode{ID: Sources, RandomEligible: true}, Eligible: true, Reason: "recent sources", FreshnessFactor: 0.25},
		{Mode: Mode{ID: Philosophy, RandomEligible: true}, Eligible: true, Reason: "source independent"},
	}
	selection, err := Choose(candidates, SelectionConfig{Weights: map[string]int{"sources": 1, "philosophy": 1}}, nil, func(max uint64) (uint64, error) {
		if max != 1_250_000 {
			t.Fatalf("total weighted range = %d, want 1250000", max)
		}
		return 300_000, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if selection.Mode == nil || selection.Mode.ID != Philosophy {
		t.Fatalf("selected mode = %#v, want philosophy after stale-source discount", selection.Mode)
	}
}

func TestChooseConsidersOutstandingResearchSignalsSeparatelyFromModeWeights(t *testing.T) {
	candidates := []Candidate{
		{Mode: Mode{ID: Questions, RandomEligible: true}, Eligible: true, SignalFactor: 1},
		{Mode: Mode{ID: Predictions, RandomEligible: true}, Eligible: true, SignalFactor: 1.5},
	}
	selection, err := Choose(candidates, SelectionConfig{Weights: map[string]int{"questions": 1, "predictions": 1}}, nil, func(max uint64) (uint64, error) {
		if max != 2_500_000 {
			t.Fatalf("total weighted range = %d, want 2500000", max)
		}
		return 1_100_000, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if selection.Mode == nil || selection.Mode.ID != Predictions {
		t.Fatalf("selected mode = %#v, want predictions with a higher research-signal factor", selection.Mode)
	}
}
