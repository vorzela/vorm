package main

import (
	"runtime/debug"
	"strings"
)

// version may be set at build time:
//
//	go build -ldflags "-X main.version=v0.2.24" ./cmd/vorm
//
// Otherwise it comes from the module version recorded by the Go toolchain.
// `go install ...@vX.Y.Z` records the GitHub tag, so the CLI reports the
// version it was actually installed from instead of a hard-coded string.
var version string

func versionString() string {
	if v := normalizeVersion(version); v != "" {
		return v
	}
	info, ok := debug.ReadBuildInfo()
	return versionFromBuildInfo(info, ok)
}

func versionFromBuildInfo(info *debug.BuildInfo, ok bool) string {
	if !ok || info == nil {
		return "devel"
	}
	if v := normalizeVersion(info.Main.Version); v != "" {
		return v
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" && len(setting.Value) >= 7 {
			return "devel+" + setting.Value[:7]
		}
	}
	return "devel"
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "(devel)" {
		return ""
	}
	return strings.TrimPrefix(v, "v")
}
