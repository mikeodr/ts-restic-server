package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/tailcfg"
)

type fakeWhoIser struct {
	who *apitype.WhoIsResponse
	err error
}

func (f fakeWhoIser) WhoIs(context.Context, string) (*apitype.WhoIsResponse, error) {
	return f.who, f.err
}

func TestNewHandler(t *testing.T) {
	alice := &apitype.WhoIsResponse{
		UserProfile: &tailcfg.UserProfile{LoginName: "alice@example.com"},
		Node:        &tailcfg.Node{ComputedName: "alices-laptop"},
	}
	aliceAdmin := &apitype.WhoIsResponse{
		UserProfile: &tailcfg.UserProfile{LoginName: "alice@example.com"},
		Node:        &tailcfg.Node{ComputedName: "alices-laptop"},
		CapMap: tailcfg.PeerCapMap{
			accessCapability: {tailcfg.RawMessage(`{"admin":true}`)},
		},
	}

	tests := []struct {
		name              string
		lc                fakeWhoIser
		requireCapability bool
		privateRepos      bool
		path              string
		wantStatus        int
		wantCalled        bool
		wantHeader        string
	}{
		{
			name:       "WhoIs error is forbidden",
			lc:         fakeWhoIser{err: errors.New("not found")},
			wantStatus: http.StatusForbidden,
		},
		{
			name:              "required capability missing is forbidden",
			lc:                fakeWhoIser{who: alice},
			requireCapability: true,
			wantStatus:        http.StatusForbidden,
		},
		{
			name:       "granted non-admin sets resolved user header",
			lc:         fakeWhoIser{who: alice},
			path:       "/alice/config",
			wantStatus: http.StatusOK,
			wantCalled: true,
			wantHeader: "alice@example.com",
		},
		{
			name:         "admin with private repos impersonates requested repo",
			lc:           fakeWhoIser{who: aliceAdmin},
			privateRepos: true,
			path:         "/somerepo/config",
			wantStatus:   http.StatusOK,
			wantCalled:   true,
			wantHeader:   "somerepo",
		},
		{
			name:         "admin with private repos but root path falls back to resolved user",
			lc:           fakeWhoIser{who: aliceAdmin},
			privateRepos: true,
			path:         "/",
			wantStatus:   http.StatusOK,
			wantCalled:   true,
			wantHeader:   "alice@example.com",
		},
		{
			name:         "admin without private repos falls back to resolved user",
			lc:           fakeWhoIser{who: aliceAdmin},
			privateRepos: false,
			path:         "/somerepo/config",
			wantStatus:   http.StatusOK,
			wantCalled:   true,
			wantHeader:   "alice@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var called bool
			var gotHeader string
			restHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				gotHeader = r.Header.Get("X-Tailscale-User")
				w.WriteHeader(http.StatusOK)
			})

			handler := newHandler(restHandler, tt.lc, tt.requireCapability, tt.privateRepos)

			path := tt.path
			if path == "" {
				path = "/"
			}
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if called != tt.wantCalled {
				t.Fatalf("restHandler called = %t, want %t", called, tt.wantCalled)
			}
			if called && gotHeader != tt.wantHeader {
				t.Fatalf("X-Tailscale-User = %q, want %q", gotHeader, tt.wantHeader)
			}
		})
	}
}
