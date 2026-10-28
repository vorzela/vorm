package main

import (
	"runtime/debug"
	"testing"
)

func TestVersionFromBuildInfoUsesModuleVersion(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v0.2.24"}}
	if got := versionFromBuildInfo(info, true); got != "0.2.24" {
		t.Fatalf("version = %q, want 0.2.24", got)
	}
}

func TestVersionFromBuildInfoFallsBackToRevision(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "6e3c250032eda9a56cca4c54b0727532a46db2c1"},
		},
	}
	if got := versionFromBuildInfo(info, true); got != "devel+6e3c250" {
		t.Fatalf("version = %q, want devel+6e3c250", got)
	}
}

func TestVersionFromBuildInfoWithoutInfo(t *testing.T) {
	if got := versionFromBuildInfo(nil, false); got != "devel" {
		t.Fatalf("version = %q, want devel", got)
	}
}

func TestNormalizeVersion(t *testing.T) {
	cases := map[string]string{
		"v0.2.24":  "0.2.24",
		"0.2.24":   "0.2.24",
		"(devel)":  "",
		"  v1.0.0": "1.0.0",
	}
	for in, want := range cases {
		if got := normalizeVersion(in); got != want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", in, got, want)
		}
	}
}
