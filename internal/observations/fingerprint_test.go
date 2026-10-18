package observations

import "testing"

func TestFingerprintMatchesSyndicatedVariants(t *testing.T) {
	one, err := Fingerprint("https://example.org/article?id=7&utm_source=a", "  Shared   Headline ", "Example News")
	if err != nil {
		t.Fatal(err)
	}
	two, err := Fingerprint("https://EXAMPLE.org/article?fbclid=b&id=7#part", "Shared headline", "Example News Syndicate")
	if err != nil {
		t.Fatal(err)
	}
	if one != two {
		t.Fatalf("syndicated fingerprints differ: %q vs %q", one, two)
	}
}

func TestFingerprintSeparatesDistinctStories(t *testing.T) {
	one, err := Fingerprint("https://example.org/article?id=7", "Headline one", "Source")
	if err != nil {
		t.Fatal(err)
	}
	two, err := Fingerprint("https://example.org/article?id=8", "Headline two", "Source")
	if err != nil {
		t.Fatal(err)
	}
	if one == two {
		t.Fatal("distinct stories share fingerprint")
	}
}
