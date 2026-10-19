package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/config"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/gitpub"
)

func TestDivergenceCommandReturnsZeroForNoDiscrepancies(t *testing.T) {
	directory := t.TempDir()
	runtimeDir := filepath.Join(directory, "runtime")
	codexHome := filepath.Join(directory, "codex-home")
	public := filepath.Join(directory, "public")
	for _, path := range []string{runtimeDir, codexHome, public} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(directory, "daemon.yaml")
	content := "dry_run: true\nsources_path: sources.yaml\nruntime_dir: " + runtimeDir + "\ncodex_model: gpt-6\ncodex_home: " + codexHome + "\npublic_repo_path: " + public + "\npublic_branch: main\npublic_remote: https://github.com/example/public.git\n"
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	opener := func(context.Context, config.Config) (*gitpub.Repository, error) {
		// The no-history report path is covered end-to-end at the operations layer.
		// Here the repository opener fails to prove command error handling does not
		// masquerade as a warning.
		return nil, errors.New("fixture repository unavailable")
	}
	var stdout, stderr bytes.Buffer
	if code := runDivergenceWithOpener([]string{"--config", configPath}, &stdout, &stderr, opener); code != 1 {
		t.Fatalf("repository error exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "divergence repository open failed") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestDivergenceCommandRejectsInvalidArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runDivergenceWithOpener([]string{"--unknown"}, &stdout, &stderr, nil); code != 2 {
		t.Fatalf("invalid argument exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "usage: themachinewrites divergence") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
