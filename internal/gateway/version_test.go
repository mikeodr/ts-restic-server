package gateway

import (
	"regexp"
	"runtime/debug"
	"testing"
)

func TestBuildVersion(t *testing.T) {
	tests := []struct {
		name     string
		stamp    string
		settings []debug.BuildSetting
		want     string
	}{
		{name: "official", stamp: "v1.2.3", want: "v1.2.3"},
		{
			name: "development",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "123456789abcdef"},
				{Key: "vcs.time", Value: "2026-08-20T10:00:00Z"},
			},
			want: "unreleased-dev20260820-123456789",
		},
		{
			name: "dirty development",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "123456789abcdef"},
				{Key: "vcs.time", Value: "2026-08-20T10:00:00Z"},
				{Key: "vcs.modified", Value: "true"},
			},
			want: "unreleased-dev20260820-123456789-dirty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.stamp != "" {
				if got := BuildVersion(test.stamp); got != test.want {
					t.Fatalf("BuildVersion() = %q, want %q", got, test.want)
				}
				return
			}
			if got := buildVersionFromInfo(baseVersion, &debug.BuildInfo{Settings: test.settings}); got != test.want {
				t.Fatalf("buildVersionFromInfo() = %q, want %q", got, test.want)
			}
		})
	}
}

// TestBuildVersionFromBuildInfo exercises BuildVersion's use of
// debug.ReadBuildInfo() (the stamp == "" path), which buildVersionFromInfo
// tests bypass by calling buildVersionFromInfo directly. The actual build
// info available under `go test` isn't controllable, so this only checks
// the result has one of the shapes BuildVersion can produce.
func TestBuildVersionFromBuildInfo(t *testing.T) {
	want := regexp.MustCompile(`^unreleased(-dev-unknown|-dev\d{8}-[0-9a-f]{1,9}(-dirty)?)$`)
	if got := BuildVersion(""); !want.MatchString(got) {
		t.Fatalf("BuildVersion(\"\") = %q, want match of %s", got, want)
	}
}
