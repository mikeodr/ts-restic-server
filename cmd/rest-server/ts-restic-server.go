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
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	restserver "github.com/restic/rest-server"
	"tailscale.com/tailcfg"
	"tailscale.com/tsnet"
)

// baseVersion is the fallback version used when a build isn't stamped by
// goreleaser, e.g. `go build`/`go install` from source. It's deliberately
// not a valid-looking release tag so it can't be mistaken for one.
const baseVersion = "unreleased"

// versionStamp is set by release builds with -ldflags.
var versionStamp string

func main() {

	authKey := flag.String("ts-authkey", os.Getenv("TS_AUTHKEY"), "Tailscale auth key to use for tsnet server")
	path := flag.String("path", filepath.Join(os.TempDir(), "restic"), "Data directory")
	appendOnly := flag.Bool("append-only", false, "If true, the rest-server will be in append-only mode")
	privateRepos := flag.Bool("private-repos", false, "If true, the rest-server will only allow access to private repositories")
	debug := flag.Bool("debug", false, "output debug information")
	maxRepoSize := flag.Int64("max-repo-size", 0, "maximum size of a repository in bytes (0 means no limit)")
	capability := flag.String("capability", "", "required Tailscale app capability for clients")
	hostName := flag.String("hostname", "restic-gw", "Tailscale hostname for the server")
	showVersion := flag.Bool("version", false, "print the version and exit")
	showVersionShort := flag.Bool("v", false, "print the version and exit")

	flag.Parse()
	if *showVersion || *showVersionShort {
		fmt.Printf("rest-server %s compiled with %v on %v/%v\n", buildVersion(), runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return
	}

	srv := &tsnet.Server{Hostname: *hostName, AuthKey: *authKey}
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
		if !hasCapability(who.CapMap, *capability) {
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

func buildVersion() string {
	if versionStamp != "" {
		return versionStamp
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return baseVersion + "-dev-unknown"
	}
	return buildVersionFromInfo(baseVersion, info)
}

func buildVersionFromInfo(base string, info *debug.BuildInfo) string {
	var revision, commitDate string
	dirty := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.time":
			if len(setting.Value) >= len("2006-01-02") {
				commitDate = strings.ReplaceAll(setting.Value[:len("2006-01-02")], "-", "")
			}
		case "vcs.modified":
			dirty = setting.Value == "true"
		}
	}

	if revision == "" || commitDate == "" {
		return base + "-dev-unknown"
	}
	if len(revision) > 9 {
		revision = revision[:9]
	}
	version := fmt.Sprintf("%s-dev%s-%s", base, commitDate, revision)
	if dirty {
		version += "-dirty"
	}
	return version
}

func hasCapability(capMap tailcfg.PeerCapMap, required string) bool {
	if required == "" {
		return true
	}
	_, granted := capMap[tailcfg.PeerCapability(required)]
	return granted
}
