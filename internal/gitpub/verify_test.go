package gitpub

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyAllowlistedChangedFiles(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"feed/latest.json", "posts/2026/10/18/01M589K0G00000000000000000.md"} {
		absolute := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := VerifyChangedFiles(context.Background(), root, []string{"feed/latest.json", "posts/2026/10/18/01M589K0G00000000000000000.md"}); err != nil {
		t.Fatal(err)
	}
	if err := VerifyChangedFiles(context.Background(), root, []string{".env"}); err == nil {
		t.Fatal("unsafe path accepted")
	}
}
