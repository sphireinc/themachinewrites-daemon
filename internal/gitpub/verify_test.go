package gitpub

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyAllowlistedChangedFiles(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"feed/latest.json"} {
		absolute := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(`{"schema_version":1,"generated_at":"2026-10-18T20:00:00Z","posts":[]}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := VerifyChangedFiles(context.Background(), root, []string{"feed/latest.json"}); err != nil {
		t.Fatal(err)
	}
	if err := VerifyChangedFiles(context.Background(), root, []string{".env"}); err == nil {
		t.Fatal("unsafe path accepted")
	}
}
