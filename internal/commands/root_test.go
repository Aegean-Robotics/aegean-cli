package commands

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	root := NewRoot(BuildInfo{Version: "0.1.0", Commit: "abcdef", Date: "2026-05-15"})

	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stdout)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{"aegean 0.1.0", "abcdef", "2026-05-15"} {
		if !strings.Contains(got, want) {
			t.Errorf("version output missing %q; got %q", want, got)
		}
	}
}

func TestRootHelpListsAllSubcommands(t *testing.T) {
	root := NewRoot(BuildInfo{Version: "test"})

	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stdout)
	root.SetArgs([]string{"--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{"login", "logout", "keys", "domains", "send", "logs", "config", "version"} {
		if !strings.Contains(got, want) {
			t.Errorf("help missing subcommand %q; got %q", want, got)
		}
	}
}
