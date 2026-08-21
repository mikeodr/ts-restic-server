package gateway

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// baseVersion is the fallback version used when a build isn't stamped by
// goreleaser, e.g. `go build`/`go install` from source. It's deliberately
// not a valid-looking release tag so it can't be mistaken for one.
const baseVersion = "unreleased"

// BuildVersion returns the version string to report for -version/-v. If
// stamp is non-empty (set via ldflags at build time) it is returned as-is;
// otherwise the version is derived from embedded VCS build info.
func BuildVersion(stamp string) string {
	if stamp != "" {
		return stamp
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return baseVersion + "-dev-unknown"
	}
	return buildVersionFromInfo(baseVersion, info)
}

func buildVersionFromInfo(base string, info *debug.BuildInfo) string {
	var revision, commitDate string
	dirty := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.time":
			if len(setting.Value) >= len("2006-01-02") {
				commitDate = strings.ReplaceAll(setting.Value[:len("2006-01-02")], "-", "")
			}
		case "vcs.modified":
			dirty = setting.Value == "true"
		}
	}

	if revision == "" || commitDate == "" {
		return base + "-dev-unknown"
	}
	if len(revision) > 9 {
		revision = revision[:9]
	}
	version := fmt.Sprintf("%s-dev%s-%s", base, commitDate, revision)
	if dirty {
		version += "-dirty"
	}
	return version
}
