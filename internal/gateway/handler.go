package gateway

import (
	"context"
	"net/http"

	"tailscale.com/client/tailscale/apitype"
)

// whoIser resolves the Tailscale identity behind a connecting peer's
// address. Satisfied by *tailscale.com/client/local.Client.
type whoIser interface {
	WhoIs(ctx context.Context, remoteAddr string) (*apitype.WhoIsResponse, error)
}

// newHandler wraps restHandler with Tailscale-identity authentication: it
// resolves the connecting peer via lc, checks its access capability, and
// sets the X-Tailscale-User header rest-server uses for ownership checks
// before delegating to restHandler.
func newHandler(restHandler http.Handler, lc whoIser, requireCapability, privateRepos bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		who, err := lc.WhoIs(r.Context(), r.RemoteAddr)
		if err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		granted, admin := checkAccess(who.CapMap, requireCapability)
		if !granted {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		r.Header.Set("X-Tailscale-User", effectiveUser(who, admin, privateRepos, r.URL.Path))
		restHandler.ServeHTTP(w, r)
	})
}
