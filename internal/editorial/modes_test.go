package editorial

import (
	"reflect"
	"testing"
)

func TestRegistryExposesEveryEditorialModeWithOperationalGuidance(t *testing.T) {
	want := []string{
		"random", "sources", "philosophy", "deep-dive", "connections", "questions",
		"predictions", "retrospective", "critique", "discovery", "historical-context",
		"thought-experiment", "explainer", "follow-up", "self-review", "corrections",
	}
	if got := IDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs() = %v, want %v", got, want)
	}
	for _, id := range want {
		mode, ok := Lookup(id)
		if !ok || string(mode.ID) != id || mode.Description == "" || mode.Instructions == "" {
			t.Errorf("mode %q lacks registry guidance: %#v, found=%t", id, mode, ok)
		}
	}
	if mode, _ := Lookup("corrections"); mode.RandomEligible {
		t.Fatal("corrections mode must never enter ordinary random selection")
	}
}

func TestLookupRejectsUnknownMode(t *testing.T) {
	if _, ok := Lookup("made-up-mode"); ok {
		t.Fatal("Lookup accepted an unregistered mode")
	}
}
