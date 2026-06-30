package main

import "testing"

func TestRootHasSubcommands(t *testing.T) {
	got := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		got[c.Name()] = true
	}
	for _, want := range []string{"inspect", "push", "rm"} {
		if !got[want] {
			t.Errorf("rootCmd missing subcommand %q", want)
		}
	}
}
