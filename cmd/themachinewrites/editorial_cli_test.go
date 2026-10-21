package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeInvocationDefaultsAndPreservesExistingCommands(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantCommand string
		wantArgs    []string
	}{
		{name: "no args starts daemon", wantCommand: "run"},
		{name: "mode flag starts daemon", args: []string{"--editorial-mode=philosophy"}, wantCommand: "run", wantArgs: []string{"--editorial-mode=philosophy"}},
		{name: "run command remains", args: []string{"run", "--editorial-mode=sources"}, wantCommand: "run", wantArgs: []string{"--editorial-mode=sources"}},
		{name: "once remains", args: []string{"once", "--config", "daemon.yaml"}, wantCommand: "once", wantArgs: []string{"--config", "daemon.yaml"}},
		{name: "help remains", args: []string{"--help"}, wantCommand: "--help", wantArgs: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, args, list, err := normalizeInvocation(tt.args)
			if err != nil || list || command != tt.wantCommand || !reflect.DeepEqual(args, tt.wantArgs) {
				t.Fatalf("normalizeInvocation(%v) = (%q, %v, %t, %v), want (%q, %v, false, nil)", tt.args, command, args, list, err, tt.wantCommand, tt.wantArgs)
			}
		})
	}
}

func TestNormalizeInvocationListsAndValidatesEditorialModes(t *testing.T) {
	command, args, list, err := normalizeInvocation([]string{"--list-editorial-modes"})
	if err != nil || !list || command != "" || len(args) != 0 {
		t.Fatalf("list invocation = (%q, %v, %t, %v)", command, args, list, err)
	}
	_, _, _, err = normalizeInvocation([]string{"--editorial-mode=not-a-mode"})
	if err == nil || !strings.Contains(err.Error(), "valid modes") || !strings.Contains(err.Error(), "philosophy") {
		t.Fatalf("invalid mode error = %v, want valid mode alternatives", err)
	}
}
