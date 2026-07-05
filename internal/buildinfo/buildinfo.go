// Package buildinfo exposes version metadata injected at build time via
// -ldflags, with sensible fallbacks to Go's embedded module version so that
// `go install`-ed builds still report something useful.
package buildinfo

import "runtime/debug"

// These are overridden at release time with:
//
//	-ldflags "-X github.com/arbazkhan971/ffgo/internal/buildinfo.Version=v1.2.3 ..."
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Resolve returns the effective version/commit/date, filling gaps from the
// binary's embedded build info (populated by `go install module@version`).
func Resolve() (version, commit, date string) {
	version, commit, date = Version, Commit, Date
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if commit == "none" && s.Value != "" {
				commit = s.Value
				if len(commit) > 12 {
					commit = commit[:12]
				}
			}
		case "vcs.time":
			if date == "unknown" && s.Value != "" {
				date = s.Value
			}
		}
	}
	return
}
