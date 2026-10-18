package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestProbeAcceptsSupportedCodexCLI(t *testing.T) {
	runner := func(_ context.Context, _ string, args ...string) ([]byte, []byte, error) {
		if len(args) > 0 && args[0] == "exec" {
			return []byte("--output-schema --output-last-message --json --sandbox"), nil, nil
		}
		return []byte("codex-cli 0.163.0\n"), nil, nil
	}
	compatibility, err := Probe(context.Background(), "codex", runner)
	if err != nil {
		t.Fatal(err)
	}
	if compatibility.Version != "0.163.0" || !compatibility.SupportsExec {
		t.Fatalf("compatibility = %#v", compatibility)
	}
}

func TestProbeRejectsUnsupportedOrMissingCLI(t *testing.T) {
	for _, tt := range []struct {
		name   string
		runner commandRunner
		want   string
	}{
		{name: "missing", runner: func(context.Context, string, ...string) ([]byte, []byte, error) {
			return nil, nil, errors.New("executable file not found")
		}, want: "not available"},
		{name: "unsupported", runner: func(context.Context, string, ...string) ([]byte, []byte, error) {
			return []byte("codex-cli 0.150.0\n"), nil, nil
		}, want: "below minimum"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Probe(context.Background(), "codex", tt.runner)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Probe() error = %v, want %q", err, tt.want)
			}
		})
	}
}
