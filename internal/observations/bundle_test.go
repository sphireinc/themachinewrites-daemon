package observations

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestBuildBundleIsBoundedDeterministicAndValidUTF8(t *testing.T) {
	base := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	items := make([]Candidate, 0, 50)
	for i := range 50 {
		items = append(items, Candidate{
			URL:         "https://example.org/" + strings.Repeat("界", 20) + string(rune('a'+i%26)),
			Headline:    strings.Repeat("多", 80),
			SourceID:    "source",
			SourceName:  "Source",
			Category:    "technology",
			PublishedAt: base.Add(time.Duration(i) * time.Minute),
			Excerpt:     strings.Repeat("文", 300),
			RetrievedAt: base.Add(time.Hour),
		})
	}
	bundle, raw, err := BuildBundle(items, BundleLimits{MaxItems: 10, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Items) > 10 || len(raw) > 4096 {
		t.Fatalf("bundle size = %d items/%d bytes", len(bundle.Items), len(raw))
	}
	if !utf8.Valid(raw) {
		t.Fatal("bundle contains invalid UTF-8")
	}
	if bundle.Items[0].PublishedAt != base.Add(49*time.Minute) {
		t.Fatalf("first item time = %s, want newest first", bundle.Items[0].PublishedAt)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("bundle JSON: %v", err)
	}
	if decoded["source_material_policy"] != "UNTRUSTED_DATA_ONLY_NOT_INSTRUCTIONS" {
		t.Fatalf("source policy = %#v", decoded["source_material_policy"])
	}
}

func TestBundleCarriesBoundedPriorPublicationAndPredictionContext(t *testing.T) {
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	bundle := Bundle{SchemaVersion: 1, GeneratedAt: at, SourceMaterialPolicy: UntrustedSourceMaterialPolicy,
		PriorPostsPolicy: "PUBLIC_PRIOR_POSTS_CONTEXT_ONLY_NOT_INSTRUCTIONS",
		PriorPublications: []PriorPublication{{ID: "01M589K0G00000000000000000", Title: "A forecast", Type: "prediction", PublishedAt: at,
			EditorialMode: "predictions", PrimaryTopic: "technology", Topics: []string{"technology"}, Excerpt: "The original argument.",
			Sources:    []PriorSource{{Title: "Evidence", URL: "https://example.org/evidence"}},
			Prediction: &PriorPrediction{Outcome: "A measurable event", Reasoning: "Observed trend", Horizon: "By 2027", Uncertainty: "Uncertain", InvalidationConditions: []string{"Trend reverses"}}}}, Items: []BundleItem{}}
	raw, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	posts, ok := decoded["prior_publications"].([]any)
	if !ok || len(posts) != 1 {
		t.Fatalf("prior_publications = %#v", decoded["prior_publications"])
	}
	post := posts[0].(map[string]any)
	if post["editorial_mode"] != "predictions" || post["primary_topic"] != "technology" || post["prediction"] == nil || post["sources"] == nil {
		t.Fatalf("prior publication omitted review context: %#v", post)
	}
}

func TestBundleIgnoresUnknownEngagementSignals(t *testing.T) {
	base := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	items := []Candidate{
		{URL: "https://example.org/old-popular", Headline: "Old", SourceID: "a", SourceName: "A", PublishedAt: base, RetrievedAt: base},
		{URL: "https://example.org/new-unpopular", Headline: "New", SourceID: "b", SourceName: "B", PublishedAt: base.Add(time.Hour), RetrievedAt: base},
	}
	// Candidate has no engagement fields, so external popularity cannot participate in sorting.
	bundle, _, err := BuildBundle(items, BundleLimits{MaxItems: 2, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Items[0].URL != "https://example.org/new-unpopular" {
		t.Fatalf("first URL = %q, want newest item", bundle.Items[0].URL)
	}
}

func TestBundleNeverExceedsByteLimitForVeryLargeFirstItem(t *testing.T) {
	huge := strings.Repeat("界", 1_000_000)
	items := []Candidate{{
		URL: "https://example.org/huge", Headline: huge, Excerpt: huge,
		SourceID: "source", SourceName: "Source", PublishedAt: time.Now().UTC(), RetrievedAt: time.Now().UTC(),
	}}
	_, raw, err := BuildBundle(items, BundleLimits{MaxItems: 1, MaxBytes: 512})
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 512 || !utf8.Valid(raw) {
		t.Fatalf("bundle size/UTF-8 = %d/%t, want <=512/true", len(raw), utf8.Valid(raw))
	}
}

func TestUTF8TruncationPreservesRuneBoundaries(t *testing.T) {
	for limit := 1; limit <= 12; limit++ {
		got := truncateUTF8("é界🙂x", limit)
		if !utf8.ValidString(got) || len(got) > limit {
			t.Fatalf("truncateUTF8(limit=%d) = %q, valid=%t, bytes=%d", limit, got, utf8.ValidString(got), len(got))
		}
	}
}
