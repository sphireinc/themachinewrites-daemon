package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunDispatchesSupportedCommands(t *testing.T) {
	for _, command := range []string{"run", "once", "validate-config"} {
		t.Run(command, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if code := run([]string{command}, &stdout, &stderr); code != 0 {
				t.Fatalf("run(%q) exit code = %d, stderr = %q", command, code, stderr.String())
			}
			if !strings.Contains(stdout.String(), command+" command not implemented") {
				t.Fatalf("stdout = %q, want command placeholder", stdout.String())
			}
		})
	}
}

func TestRunHelpListsEverySubcommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help exit code = %d, stderr = %q", code, stderr.String())
	}
	for _, command := range []string{"run", "once", "validate-config"} {
		if !strings.Contains(stdout.String(), command) {
			t.Errorf("help output %q does not mention %q", stdout.String(), command)
		}
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"unknown"}, &stdout, &stderr); code == 0 {
		t.Fatal("unknown command exited zero")
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q, want unknown-command message", stderr.String())
	}
}
