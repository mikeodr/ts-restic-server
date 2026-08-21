package main

import (
	"runtime/debug"
	"testing"

	"tailscale.com/client/tailscale/apitype"
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

func TestResolveUser(t *testing.T) {
	tests := []struct {
		name string
		who  *apitype.WhoIsResponse
		want string
	}{
		{
			name: "regular user",
			who: &apitype.WhoIsResponse{
				UserProfile: &tailcfg.UserProfile{LoginName: "alice@example.com"},
				Node:        &tailcfg.Node{ComputedName: "alices-laptop"},
			},
			want: "alice@example.com",
		},
		{
			name: "tagged device reports synthetic login name",
			who: &apitype.WhoIsResponse{
				UserProfile: &tailcfg.UserProfile{LoginName: "tagged-devices"},
				Node:        &tailcfg.Node{ComputedName: "db-01"},
			},
			want: "db-01",
		},
		{
			name: "empty login name falls back to node name",
			who: &apitype.WhoIsResponse{
				UserProfile: &tailcfg.UserProfile{LoginName: ""},
				Node:        &tailcfg.Node{ComputedName: "db-01"},
			},
			want: "db-01",
		},
		{
			name: "no fallback available",
			who: &apitype.WhoIsResponse{
				UserProfile: &tailcfg.UserProfile{LoginName: ""},
				Node:        &tailcfg.Node{ComputedName: ""},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveUser(tt.who); got != tt.want {
				t.Errorf("resolveUser() = %q, want %q", got, tt.want)
			}
		})
	}
}
