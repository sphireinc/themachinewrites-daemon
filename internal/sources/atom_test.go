package sources

import (
	"testing"
	"time"
)

func TestParseAtomNormalizesNamespacedEntriesAndLinks(t *testing.T) {
	fixture := `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Example Atom</title>
  <entry>
    <id>tag:example.org,2026:one</id>
    <title>Atom headline</title>
    <updated>2026-10-18T21:00:00Z</updated>
    <published>2026-10-18T20:00:00Z</published>
    <link rel="alternate" href="https://example.org/atom-one"/>
    <summary type="html">&lt;p&gt;Atom &amp; summary.&lt;/p&gt;</summary>
  </entry>
  <entry>
    <id>tag:example.org,2026:two</id>
    <title>No summary</title>
    <updated>2026-10-18T22:00:00Z</updated>
    <link href="https://example.org/atom-two"/>
  </entry>
</feed>`
	retrieved := time.Date(2026, 10, 18, 23, 0, 0, 0, time.UTC)
	items, err := ParseAtom([]byte(fixture), Source{ID: "atom", Name: "Example Atom"}, retrieved)
	if err != nil {
		t.Fatalf("ParseAtom() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("item count = %d, want 2", len(items))
	}
	if items[0].URL != "https://example.org/atom-one" || items[0].Excerpt != "Atom & summary." ||
		!items[0].PublishedAt.Equal(time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)) {
		t.Fatalf("first item = %#v", items[0])
	}
	if items[1].URL != "https://example.org/atom-two" || items[1].Excerpt != "" ||
		!items[1].PublishedAt.Equal(time.Date(2026, 10, 18, 22, 0, 0, 0, time.UTC)) {
		t.Fatalf("second item = %#v", items[1])
	}
}
