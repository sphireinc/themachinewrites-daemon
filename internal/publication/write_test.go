package publication

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteRenderedPostCreatesOnlyCanonicalFile(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	post, err := Render(validProposal(), RenderInput{
		ID: "01M589K0G00000000000000000", PublishedAt: at, ModelIdentifier: "test", PromptVersion: "p1", IndependentSources: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteRenderedPost(root, post); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(post.Path)))
	if err != nil || string(content) != string(post.Markdown) {
		t.Fatalf("written content = %q, %v", content, err)
	}
	if err := WriteRenderedPost(root, post); err != nil {
		t.Fatalf("idempotent retry rejected: %v", err)
	}
	changed := post
	changed.Markdown = append(append([]byte(nil), post.Markdown...), []byte("changed")...)
	if err := WriteRenderedPost(root, changed); err == nil {
		t.Fatal("conflicting duplicate write accepted")
	}
	for _, unsafe := range []string{"../outside.md", "/tmp/outside.md", `posts\escape.md`} {
		copy := post
		copy.Path = unsafe
		if err := WriteRenderedPost(root, copy); err == nil {
			t.Errorf("unsafe path accepted: %s", unsafe)
		}
	}
}
