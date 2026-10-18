package prompts

import (
	"strings"
	"testing"
)

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
