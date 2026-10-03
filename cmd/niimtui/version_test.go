package main

import "testing"

func TestVersionCommandsPrintVersionWithoutStartingTUI(t *testing.T) {
	for _, arg := range []string{"--version", "version", "-v"} {
		output, err := captureCLI(t, arg)
		if err != nil {
			t.Fatalf("run(%q) error = %v", arg, err)
		}
		if want := "niimtui dev (commit unknown, built unknown)\n"; output != want {
			t.Fatalf("run(%q) output = %q, want %q", arg, output, want)
		}
	}
}

func TestVersionStringUsesInjectedMetadata(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	t.Cleanup(func() { version, commit, date = oldVersion, oldCommit, oldDate })
	version, commit, date = "v0.1.0", "abc1234", "2026-09-28"

	if got, want := versionString(), "niimtui v0.1.0 (commit abc1234, built 2026-09-28)"; got != want {
		t.Fatalf("versionString() = %q, want %q", got, want)
	}
}
