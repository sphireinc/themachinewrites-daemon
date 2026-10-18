package observations

import "testing"

func TestCanonicalizeURLRemovesTrackingAndFragments(t *testing.T) {
	variants := []string{
		"https://Example.org/article?id=7&utm_source=feed&utm_medium=rss#section",
		"https://example.org/article?id=7&gclid=tracking",
		"https://example.org/article?fbclid=social&id=7",
	}
	for _, raw := range variants {
		got, err := CanonicalizeURL(raw)
		if err != nil {
			t.Fatalf("CanonicalizeURL(%q) error = %v", raw, err)
		}
		if got != "https://example.org/article?id=7" {
			t.Errorf("CanonicalizeURL(%q) = %q, want shared canonical URL", raw, got)
		}
	}
}

func TestCanonicalizeURLDoesNotMergeDistinctArticles(t *testing.T) {
	a, err := CanonicalizeURL("https://example.org/article?id=7&utm_campaign=a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := CanonicalizeURL("https://example.org/article?id=8&utm_campaign=b")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("distinct articles merged to %q", a)
	}
}
