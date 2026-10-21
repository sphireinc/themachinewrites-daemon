package prompts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionForModeBindsModeAndInstructions(t *testing.T) {
	bundle, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	philosophy, err := VersionForMode(bundle, "philosophy", "Explore ideas.")
	if err != nil {
		t.Fatal(err)
	}
	other, err := VersionForMode(bundle, "sources", "Explore ideas.")
	if err != nil {
		t.Fatal(err)
	}
	changed, err := VersionForMode(bundle, "philosophy", "Explore different ideas.")
	if err != nil {
		t.Fatal(err)
	}
	if philosophy == bundle.Version || philosophy == other || philosophy == changed {
		t.Fatal("mode prompt version did not bind its complete trusted instruction set")
	}
}

func TestNonpublicationPromptExampleMatchesRequiredProposalShape(t *testing.T) {
	bundle, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(bundle.Text, `{"action":"defer"`)
	if start < 0 {
		t.Fatal("editorial prompt has no nonpublication example")
	}
	end := strings.Index(bundle.Text[start:], "\n")
	if end < 0 {
		t.Fatal("nonpublication example has no line boundary")
	}
	var example map[string]any
	if err := json.Unmarshal([]byte(bundle.Text[start:start+end]), &example); err != nil {
		t.Fatalf("nonpublication example is not JSON: %v", err)
	}
	for _, key := range []string{"editorial_mode", "primary_topic", "factuality", "correction_error", "corrected_information", "prediction", "confidence_rationale"} {
		if value, ok := example[key]; !ok || value != nil {
			t.Errorf("nonpublication example %q = %#v, want required null field", key, value)
		}
	}
	if reviews, ok := example["prediction_reviews"].([]any); !ok || len(reviews) != 0 {
		t.Errorf("nonpublication prediction_reviews = %#v, want empty array", example["prediction_reviews"])
	}
}

func TestVersionedBundleIsStableAndOrdersAssets(t *testing.T) {
	first, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	second, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != second.Version || len(first.Version) != 71 || !strings.HasPrefix(first.Version, "sha256:") {
		t.Fatalf("versions = %q/%q", first.Version, second.Version)
	}
	if !strings.Contains(first.Text, "# Editorial identity") ||
		!strings.Contains(first.Text, "Choose exactly one action") ||
		!strings.Contains(first.Text, "# Source trust rules") ||
		!strings.Contains(first.Text, "# Citation and uncertainty rules") {
		t.Fatalf("bundle missing ordered assets:\n%s", first.Text)
	}
	if !strings.Contains(first.Text, "defer") || !strings.Contains(first.Text, "no post quota") {
		t.Fatalf("bundle lacks no-publication autonomy:\n%s", first.Text)
	}
	if !strings.Contains(first.Text, "consciousness") || !strings.Contains(first.Text, "untrusted evidence") {
		t.Fatalf("bundle lacks identity/trust boundaries:\n%s", first.Text)
	}
}

func TestRuntimeAssetsMatchRepositoryPromptSources(t *testing.T) {
	for _, name := range orderedAssets {
		runtime, err := promptFiles.ReadFile("assets/" + name)
		if err != nil {
			t.Fatal(err)
		}
		source, err := os.ReadFile(filepath.Join("..", "..", "prompts", name))
		if err != nil {
			t.Fatal(err)
		}
		if string(runtime) != string(source) {
			t.Fatalf("runtime asset %s differs from repository prompt source", name)
		}
	}
}

func TestVersionChangesWhenAssetChanges(t *testing.T) {
	assets := map[string][]byte{
		"identity.md":     []byte("identity"),
		"editorial.md":    []byte("editorial"),
		"source-trust.md": []byte("source"),
		"citation.md":     []byte("citation"),
	}
	one, err := versionAssets(assets)
	if err != nil {
		t.Fatal(err)
	}
	assets["citation.md"] = []byte("citation changed")
	two, err := versionAssets(assets)
	if err != nil {
		t.Fatal(err)
	}
	if one == two {
		t.Fatal("version did not change when an asset changed")
	}
}
