// Command restic-gw serves an embedded rest-server over Tailscale HTTPS.
//
// The gateway authenticates requests using the Tailscale identity of the
// connecting peer and passes that identity to rest-server through the
// X-Tailscale-User header. Repository data is stored in the directory given
// by -path, which defaults to os.TempDir()/restic.
//
// The -ts-authkey flag defaults to the TS_AUTHKEY environment variable.
// Additional rest-server behavior can be configured with -append-only,
// -private-repos, -debug, and -max-repo-size.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"

	restserver "github.com/restic/rest-server"
	"tailscale.com/tsnet"
)

func main() {

	authKey := flag.String("ts-authkey", os.Getenv("TS_AUTHKEY"), "Tailscale auth key to use for tsnet server")
	path := flag.String("path", filepath.Join(os.TempDir(), "restic"), "Data directory")
	appendOnly := flag.Bool("append-only", false, "If true, the rest-server will be in append-only mode")
	privateRepos := flag.Bool("private-repos", false, "If true, the rest-server will only allow access to private repositories")
	debug := flag.Bool("debug", false, "output debug information")
	maxRepoSize := flag.Int64("max-repo-size", 0, "maximum size of a repository in bytes (0 means no limit)")

	flag.Parse()

	srv := &tsnet.Server{Hostname: "restic-gw", AuthKey: *authKey}
	defer srv.Close()
	ln, err := srv.ListenTLS("tcp", ":443")
	if err != nil {
		log.Fatal(err)
	}
	lc, err := srv.LocalClient()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Data directory: %s", *path)
	restHandler, err := restserver.NewHandler(&restserver.Server{
		Path:              *path,
		AppendOnly:        *appendOnly,
		PrivateRepos:      *privateRepos,
		ProxyAuthUsername: "X-Tailscale-User",
		Debug:             *debug,
		MaxRepoSize:       *maxRepoSize,
	})
	if err != nil {
		log.Fatal(err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		who, err := lc.WhoIs(r.Context(), r.RemoteAddr)
		if err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		// For tagged devices, who.UserProfile.LoginName is empty;
		// use the node name instead.
		user := who.UserProfile.LoginName
		if user == "" {
			user = who.Node.ComputedName // e.g. "db-01" or "tag:prod"
		}

		r.Header.Set("X-Tailscale-User", user)
		log.Print(r)
		restHandler.ServeHTTP(w, r)
	})

	if err := http.Serve(ln, handler); err != nil {
		log.Fatal(err)
	}
}
