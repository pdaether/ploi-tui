package main

import (
	"bytes"
	"strings"
	"testing"
)

func runCommand(t *testing.T, args ...string) string {
	t.Helper()

	var out bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute %v: %v", args, err)
	}

	return out.String()
}

func TestVersionOutput(t *testing.T) {
	out := runCommand(t, "version")
	if !strings.Contains(out, "ploi-tui") || !strings.Contains(out, version) {
		t.Fatalf("unexpected version output: %q", out)
	}
}

func TestHelpListsSubcommands(t *testing.T) {
	out := runCommand(t, "--help")
	for _, want := range []string{"connect", "logout", "version"} {
		if !strings.Contains(out, want) {
			t.Errorf("--help output missing %q:\n%s", want, out)
		}
	}
}
