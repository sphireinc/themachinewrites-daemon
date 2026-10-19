package publication

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanRenderedPostBlocksCredentialCanaries(t *testing.T) {
	post, err := Render(validProposal(), RenderInput{
		ID: "01M589K0G00000000000000000", PublishedAt: time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC), ModelIdentifier: "test-model", PromptVersion: "sha256:test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ScanRenderedPost(post.Markdown); err != nil {
		t.Fatalf("clean post rejected: %v", err)
	}
	canaries := []string{
		"github_pat_11ABCdefGHIJKLMNOPQRSTUVWXYZ0123456789",
		"sk-proj-AbCdEfGhIjKlMnOpQrStUvWxYz0123456789",
		"AKIAABCDEFGHIJKLMNOP",
		"xoxb-123456789012-abcdefghijklmnop",
		"-----BEGIN OPENSSH PRIVATE KEY-----",
		"Bearer abcdefghijklmnopqrstuvwxyz0123456789",
	}
	for _, canary := range canaries {
		tainted := append(append([]byte(nil), post.Markdown...), "\n"+canary+"\n"...)
		if err := ScanRenderedPost(tainted); err == nil || !strings.Contains(err.Error(), "quarantine") {
			t.Errorf("canary %q: error = %v", canary, err)
		}
	}
}

func TestQuarantineRenderedPostPersistsPrivateCopy(t *testing.T) {
	root := filepath.Join(t.TempDir(), "quarantine")
	at := time.Date(2026, 10, 18, 20, 1, 2, 3, time.UTC)
	content := []byte("sensitive candidate")
	path, err := QuarantineRenderedPost(root, "01M589K0G00000000000000000", content, at)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, root+string(filepath.Separator)) {
		t.Fatalf("quarantine path %q outside root", path)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(content) {
		t.Fatalf("quarantine content = %q, %v", got, err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("quarantine mode = %v, %v", info.Mode().Perm(), err)
	}
	if _, err := QuarantineRenderedPost(root, "../escape", content, at); err == nil {
		t.Fatal("unsafe quarantine post ID accepted")
	}
}

func TestSecretInBodyPreventsCanonicalRender(t *testing.T) {
	proposal := validProposal()
	proposal.Body += "\n\nLeaked key: sk-proj-AbCdEfGhIjKlMnOpQrStUvWxYz0123456789"
	_, err := Render(proposal, RenderInput{
		ID: "01M589K0G00000000000000000", PublishedAt: time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC),
		ModelIdentifier: "test-model", PromptVersion: "sha256:test",
	})
	if err == nil || !strings.Contains(err.Error(), "quarantine") {
		t.Fatalf("Render() error = %v, want secret-scan rejection", err)
	}
}
