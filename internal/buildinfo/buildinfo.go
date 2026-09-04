// Package buildinfo exposes a human-readable build identifier for the running
// binary: the release tag plus commits-since-tag and short hash when built from
// a git checkout (e.g. "v0.9.0-5-gdb40e9f"), or "devel" when nothing is known.
//
// The value is baked in by version_gen.go, which `go generate` derives from
// `git describe`. When that file is stale or absent the VCS stamp the Go
// toolchain embeds is used as a fallback.
package buildinfo

//go:generate go run generate.go

import "runtime/debug"

// version is set by the generated version_gen.go (init). Empty means "fall back
// to the toolchain VCS stamp".
var version string

// Version returns the build identifier, always non-empty.
func Version() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		var rev string
		var dirty bool
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				dirty = s.Value == "true"
			}
		}
		if len(rev) > 12 {
			rev = rev[:12]
		}
		if rev != "" {
			if dirty {
				return rev + "-dirty"
			}
			return rev
		}
	}
	return "devel"
}
