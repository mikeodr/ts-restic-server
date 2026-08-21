// Command restic-gw serves an embedded rest-server over Tailscale HTTPS.
//
// The gateway authenticates requests using the Tailscale identity of the
// connecting peer and passes that identity to rest-server through the
// X-Tailscale-User header. If -require-capability is set, peers must hold a
// Tailscale grant for the "restic.net/cap/access" capability to use the
// server; regardless of that flag, a grant with "admin": true always allows
// access to any repository under -private-repos. Repository data is stored
// in the directory given by -path, which defaults to os.TempDir()/restic.
//
// The -ts-authkey flag defaults to the TS_AUTHKEY environment variable.
// Additional rest-server behavior can be configured with -append-only,
// -private-repos, -debug, and -max-repo-size.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mikeodr/ts-restic-server/internal/gateway"
)

// versionStamp is set by release builds with -ldflags.
var versionStamp string

func main() {
	authKey := flag.String("ts-authkey", os.Getenv("TS_AUTHKEY"), "Tailscale auth key to use for tsnet server")
	path := flag.String("path", filepath.Join(os.TempDir(), "restic"), "Data directory")
	appendOnly := flag.Bool("append-only", false, "If true, the rest-server will be in append-only mode")
	privateRepos := flag.Bool("private-repos", false, "If true, the rest-server will only allow access to private repositories")
	debug := flag.Bool("debug", false, "output debug information")
	maxRepoSize := flag.Int64("max-repo-size", 0, "maximum size of a repository in bytes (0 means no limit)")
	requireCapability := flag.Bool("require-capability", false, "if true, clients must hold the restic.net/cap/access capability to use the server")
	hostName := flag.String("hostname", "restic-gw", "Tailscale hostname for the server")
	showVersion := flag.Bool("version", false, "print the version and exit")
	showVersionShort := flag.Bool("v", false, "print the version and exit")

	flag.Parse()
	if *showVersion || *showVersionShort {
		fmt.Printf("rest-server %s compiled with %v on %v/%v\n", gateway.BuildVersion(versionStamp), runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return
	}

	cfg := gateway.Config{
		AuthKey:           *authKey,
		Path:              *path,
		AppendOnly:        *appendOnly,
		PrivateRepos:      *privateRepos,
		Debug:             *debug,
		MaxRepoSize:       *maxRepoSize,
		RequireCapability: *requireCapability,
		Hostname:          *hostName,
	}
	if err := gateway.Run(cfg); err != nil {
		log.Fatal(err)
	}
}
