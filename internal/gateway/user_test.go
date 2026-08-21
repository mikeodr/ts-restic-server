package gateway

import (
	"testing"

	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/tailcfg"
)

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

func TestEffectiveUser(t *testing.T) {
	who := &apitype.WhoIsResponse{
		UserProfile: &tailcfg.UserProfile{LoginName: "alice@example.com"},
		Node:        &tailcfg.Node{ComputedName: "alices-laptop"},
	}

	tests := []struct {
		name         string
		admin        bool
		privateRepos bool
		urlPath      string
		want         string
	}{
		{
			name:         "admin with private repos impersonates requested repo owner",
			admin:        true,
			privateRepos: true,
			urlPath:      "/somerepo/config",
			want:         "somerepo",
		},
		{
			name:         "admin with private repos but root path falls back to resolveUser",
			admin:        true,
			privateRepos: true,
			urlPath:      "/",
			want:         "alice@example.com",
		},
		{
			name:         "non-admin always falls back to resolveUser",
			admin:        false,
			privateRepos: true,
			urlPath:      "/somerepo/config",
			want:         "alice@example.com",
		},
		{
			name:         "admin without private repos falls back to resolveUser",
			admin:        true,
			privateRepos: false,
			urlPath:      "/somerepo/config",
			want:         "alice@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := effectiveUser(who, tt.admin, tt.privateRepos, tt.urlPath); got != tt.want {
				t.Errorf("effectiveUser() = %q, want %q", got, tt.want)
			}
		})
	}
}
