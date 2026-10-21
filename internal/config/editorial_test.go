package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/editorial"
)

func TestEditorialDefaultsToRandomWithPositiveRegularModeWeights(t *testing.T) {
	cfg := Default().Editorial
	if cfg.Mode != string(editorial.Random) {
		t.Fatalf("default mode = %q, want random", cfg.Mode)
	}
	if len(cfg.Weights) != 14 {
		t.Fatalf("default weights contain %d modes, want 14 named regular modes", len(cfg.Weights))
	}
	for _, mode := range editorial.RegularModes() {
		if cfg.Weights[string(mode.ID)] <= 0 {
			t.Errorf("default weight for %q = %d, want positive", mode.ID, cfg.Weights[string(mode.ID)])
		}
	}
	if _, ok := cfg.Weights[string(editorial.Corrections)]; ok {
		t.Fatal("corrections must not have a random-selection weight")
	}
}

func TestEditorialWeightConfigurationAllowsZeroAndRejectsInvalidEntries(t *testing.T) {
	valid := Default().Editorial
	valid.Weights[string(editorial.Philosophy)] = 0
	if err := valid.Validate(); err != nil {
		t.Fatalf("zero weight should be valid: %v", err)
	}

	for name, weight := range map[string]int{"negative": -1, "excessive": 1_000_001} {
		bad := Default().Editorial
		bad.Weights[string(editorial.Philosophy)] = weight
		if err := bad.Validate(); err == nil {
			t.Errorf("%s weight %d was accepted", name, weight)
		}
	}
	bad := Default().Editorial
	bad.Weights[string(editorial.Corrections)] = 5
	if err := bad.Validate(); err == nil {
		t.Error("corrections weight was accepted")
	}
	bad = Default().Editorial
	bad.Weights["unregistered"] = 1
	if err := bad.Validate(); err == nil {
		t.Error("unknown weight key was accepted")
	}
}

func TestLoadEnvyUsesDefaultsAndAcceptsConfiguredZeroWeight(t *testing.T) {
	directory := t.TempDir()
	codexHome := filepath.Join(directory, "codex")
	if err := os.Mkdir(codexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "config.yaml")
	content := "dry_run: true\ncodex_model: gpt-6\ncodex_home: " + codexHome + "\npublic_repo_path: ./public\npublic_remote: https://github.com/example/public.git\neditorial:\n  weights:\n    philosophy: 0\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadEnvy(path, func(string) string { return "" })
	if err != nil {
		t.Fatalf("LoadEnvy() error = %v", err)
	}
	if cfg.Editorial.Mode != string(editorial.Random) || cfg.Editorial.Weights[string(editorial.Philosophy)] != 0 || cfg.Editorial.Weights[string(editorial.Sources)] <= 0 {
		t.Fatalf("editorial settings were not defaulted/overridden as expected: %#v", cfg.Editorial)
	}
}
