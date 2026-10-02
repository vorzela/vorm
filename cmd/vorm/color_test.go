package main

import "testing"

func TestPadDots(t *testing.T) {
	got := padDots("  create_users", "12ms DONE", 40)
	if len(got) < 40 {
		t.Fatalf("too short: %q", got)
	}
	if got[:len("  create_users")] != "  create_users" {
		t.Fatalf("prefix: %q", got)
	}
	if got[len(got)-len("12ms DONE"):] != "12ms DONE" {
		t.Fatalf("suffix: %q", got)
	}
}

func TestColorDisabledByEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("VORM_NO_COLOR", "")
	if colorEnabled() {
		t.Fatal("NO_COLOR should disable color")
	}
}
