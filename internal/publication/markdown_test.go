package publication

import "testing"

func TestValidateMarkdownRejectsExecutableContent(t *testing.T) {
	fixtures := []string{
		`<script>alert(1)</script>`,
		`<img src=x onerror=alert(1)>`,
		`[click](javascript:alert(1))`,
		`![x](data:image/svg+xml,<svg onload=alert(1)>)`,
		"---\nid: 01M589K0G00000000000000000\n---",
		"<iframe src=\"https://example.org\"></iframe>",
	}
	for _, fixture := range fixtures {
		if err := ValidateMarkdown(fixture); err == nil {
			t.Errorf("unsafe Markdown accepted: %q", fixture)
		}
	}
}

func TestValidateMarkdownAcceptsSafeMarkdown(t *testing.T) {
	safe := "# Heading\n\nA paragraph with [a safe link](https://example.org) and **emphasis**."
	if err := ValidateMarkdown(safe); err != nil {
		t.Fatalf("safe Markdown rejected: %v", err)
	}
}
