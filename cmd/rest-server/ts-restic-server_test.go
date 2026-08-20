package main

import (
	"runtime/debug"
	"testing"

	"tailscale.com/tailcfg"
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
			oldStamp := versionStamp
			versionStamp = test.stamp
			t.Cleanup(func() {
				versionStamp = oldStamp
			})

			if test.stamp != "" {
				if got := buildVersion(); got != test.want {
					t.Fatalf("buildVersion() = %q, want %q", got, test.want)
				}
				return
			}
			if got := buildVersionFromInfo(baseVersion, &debug.BuildInfo{Settings: test.settings}); got != test.want {
				t.Fatalf("buildVersionFromInfo() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestHasCapability(t *testing.T) {
	const capability = "example.com/restic-backup"

	tests := []struct {
		name    string
		capMap  tailcfg.PeerCapMap
		require string
		want    bool
	}{
		{name: "not required", require: "", want: true},
		{name: "granted", capMap: tailcfg.PeerCapMap{tailcfg.PeerCapability(capability): {}}, require: capability, want: true},
		{name: "not granted", require: capability, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := hasCapability(test.capMap, test.require); got != test.want {
				t.Fatalf("hasCapability() = %t, want %t", got, test.want)
			}
		})
	}
}
