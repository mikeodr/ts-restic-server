// Package gateway serves an embedded rest-server over Tailscale HTTPS.
//
// It authenticates requests using the Tailscale identity of the connecting
// peer and passes that identity to rest-server through the
// X-Tailscale-User header. If Config.RequireCapability is set, peers must
// hold a Tailscale grant for the "restic.net/cap/access" capability to use
// the server; regardless of that flag, a grant with "admin": true always
// allows access to any repository under Config.PrivateRepos.
package gateway

import (
	"fmt"
	"log"
	"net/http"

	restserver "github.com/restic/rest-server"
	"tailscale.com/tsnet"
)

// Config holds the runtime configuration for the gateway server.
type Config struct {
	AuthKey           string
	Path              string
	AppendOnly        bool
	PrivateRepos      bool
	Debug             bool
	MaxRepoSize       int64
	RequireCapability bool
	Hostname          string
}

// Run starts the tsnet-backed rest-server gateway and blocks serving
// requests until it exits.
func Run(cfg Config) error {
	srv := &tsnet.Server{Hostname: cfg.Hostname, AuthKey: cfg.AuthKey}
	defer srv.Close()

	ln, err := srv.ListenTLS("tcp", ":443")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	lc, err := srv.LocalClient()
	if err != nil {
		return fmt.Errorf("local client: %w", err)
	}

	log.Printf("Data directory: %s", cfg.Path)
	restHandler, err := restserver.NewHandler(&restserver.Server{
		Path:              cfg.Path,
		AppendOnly:        cfg.AppendOnly,
		PrivateRepos:      cfg.PrivateRepos,
		ProxyAuthUsername: "X-Tailscale-User",
		Debug:             cfg.Debug,
		MaxRepoSize:       cfg.MaxRepoSize,
	})
	if err != nil {
		return fmt.Errorf("rest-server handler: %w", err)
	}

	handler := newHandler(restHandler, lc, cfg.RequireCapability, cfg.PrivateRepos)
	if err := http.Serve(ln, handler); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
