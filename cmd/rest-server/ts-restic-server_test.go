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

func TestCheckAccess(t *testing.T) {
	tests := []struct {
		name        string
		capMap      tailcfg.PeerCapMap
		required    bool
		wantGranted bool
		wantAdmin   bool
	}{
		{name: "not required, no capability", required: false, wantGranted: true},
		{name: "required, no capability", required: true, wantGranted: false},
		{
			name:        "required, granted, no grant body",
			capMap:      tailcfg.PeerCapMap{accessCapability: {}},
			required:    true,
			wantGranted: true,
		},
		{
			name: "required, granted without admin",
			capMap: tailcfg.PeerCapMap{
				accessCapability: {tailcfg.RawMessage(`{"admin":false}`)},
			},
			required:    true,
			wantGranted: true,
		},
		{
			name: "required, granted with admin",
			capMap: tailcfg.PeerCapMap{
				accessCapability: {tailcfg.RawMessage(`{"admin":true}`)},
			},
			required:    true,
			wantGranted: true,
			wantAdmin:   true,
		},
		{
			name: "not required but admin grant still recognized",
			capMap: tailcfg.PeerCapMap{
				accessCapability: {tailcfg.RawMessage(`{"admin":true}`)},
			},
			required:    false,
			wantGranted: true,
			wantAdmin:   true,
		},
		{
			name: "admin among multiple grants",
			capMap: tailcfg.PeerCapMap{
				accessCapability: {
					tailcfg.RawMessage(`{}`),
					tailcfg.RawMessage(`{"admin":true}`),
				},
			},
			required:    true,
			wantGranted: true,
			wantAdmin:   true,
		},
		{
			name: "malformed grant body still grants but not admin",
			capMap: tailcfg.PeerCapMap{
				accessCapability: {tailcfg.RawMessage(`not json`)},
			},
			required:    true,
			wantGranted: true,
		},
		{
			name: "admin entry survives a malformed sibling entry",
			capMap: tailcfg.PeerCapMap{
				accessCapability: {
					tailcfg.RawMessage(`{"admin":true}`),
					tailcfg.RawMessage(`{"admin":"true"}`), // e.g. ACL typo: string, not bool
				},
			},
			required:    true,
			wantGranted: true,
			wantAdmin:   true,
		},
		{
			name: "malformed entry doesn't grant admin on its own",
			capMap: tailcfg.PeerCapMap{
				accessCapability: {
					tailcfg.RawMessage(`{"admin":"true"}`), // e.g. ACL typo: string, not bool
					tailcfg.RawMessage(`{}`),
				},
			},
			required:    true,
			wantGranted: true,
			wantAdmin:   false,
		},
		{
			name: "unrelated capability doesn't grant required access",
			capMap: tailcfg.PeerCapMap{
				tailcfg.PeerCapability("example.com/other"): {},
			},
			required: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotGranted, gotAdmin := checkAccess(test.capMap, test.required)
			if gotGranted != test.wantGranted || gotAdmin != test.wantAdmin {
				t.Fatalf("checkAccess() = (%t, %t), want (%t, %t)", gotGranted, gotAdmin, test.wantGranted, test.wantAdmin)
			}
		})
	}
}

func TestFirstPathSegment(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/alice/config", want: "alice"},
		{path: "/alice", want: "alice"},
		{path: "/", want: ""},
		{path: "", want: ""},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			if got := firstPathSegment(test.path); got != test.want {
				t.Fatalf("firstPathSegment(%q) = %q, want %q", test.path, got, test.want)
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
