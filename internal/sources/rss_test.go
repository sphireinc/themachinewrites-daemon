package sources

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseRSSNormalizesItemsAndStripsMarkup(t *testing.T) {
	fixture := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel>
  <title>Example News</title>
  <item>
    <guid isPermaLink="true">https://example.org/a</guid>
    <title>First &amp; useful headline</title>
    <link>https://example.org/a</link>
    <pubDate>Sun, 18 Oct 2026 20:00:00 GMT</pubDate>
    <description><![CDATA[<p>Brief <strong>excerpt</strong>.</p><script>alert(1)</script>]]></description>
  </item>
  <item>
    <guid>item-two</guid>
    <title>Second headline</title>
    <link>https://example.org/b?utm_source=feed</link>
    <pubDate>Sun, 18 Oct 2026 21:00:00 GMT</pubDate>
    <description>Plain excerpt.</description>
  </item>
</channel></rss>`

	items, err := ParseRSS([]byte(fixture), Source{
		ID: "example", Name: "Example News", Category: "technology",
	}, time.Date(2026, 10, 18, 22, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ParseRSS() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("item count = %d, want 2", len(items))
	}
	if items[0].GUID != "https://example.org/a" || items[0].Headline != "First & useful headline" ||
		items[0].URL != "https://example.org/a" || items[0].SourceID != "example" ||
		items[0].SourceName != "Example News" || items[0].Category != "technology" ||
		items[0].Excerpt != "Brief excerpt." ||
		!items[0].PublishedAt.Equal(time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)) {
		t.Fatalf("first observation = %#v", items[0])
	}
}

func TestParseRSSRejectsUnsafeItemURLsAndBoundsItems(t *testing.T) {
	itemsXML := func(count int) string {
		var builder strings.Builder
		builder.WriteString(`<rss><channel>`)
		for i := range count {
			fmt.Fprintf(&builder, `<item><guid>id-%d</guid><title>Item %d</title><link>https://example.org/%d</link><pubDate>Sun, 18 Oct 2026 20:00:00 GMT</pubDate><description>Excerpt</description></item>`, i, i, i)
		}
		builder.WriteString(`</channel></rss>`)
		return builder.String()
	}
	items, err := ParseRSS([]byte(itemsXML(101)), Source{ID: "s", Name: "Source"}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 100 {
		t.Fatalf("bounded item count = %d, want 100", len(items))
	}

	unsafe := `<rss><channel><item><guid>x</guid><title>Bad</title><link>javascript:alert(1)</link><description>bad</description></item></channel></rss>`
	items, err = ParseRSS([]byte(unsafe), Source{ID: "s", Name: "Source"}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("unsafe item count = %d, want 0", len(items))
	}
}
