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
	"encoding/json"
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
	"tailscale.com/client/tailscale/apitype"
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
	requireCapability := flag.Bool("require-capability", false, "if true, clients must hold the restic.net/cap/access capability to use the server")
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
		granted, admin := checkAccess(who.CapMap, *requireCapability)
		if !granted {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		r.Header.Set("X-Tailscale-User", effectiveUser(who, admin, *privateRepos, r.URL.Path))
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

// resolveUser derives the identity to record for the connecting peer.
//
// For tagged devices, who.UserProfile.LoginName isn't empty -- Tailscale
// sets it to the synthetic value "tagged-devices".
// Fall back to the node's real hostname in that case.
func resolveUser(who *apitype.WhoIsResponse) string {
	user := who.UserProfile.LoginName
	if user == "" || user == "tagged-devices" {
		user = who.Node.ComputedName // e.g. "db-01"
	}
	return user
}

// effectiveUser returns the identity to present to rest-server as
// X-Tailscale-User. rest-server's private-repos check is just
// folderPath[0] == username, so an admin peer is impersonated as the
// requested repository's owner: this is the seam that translates the
// gateway's admin capability into rest-server's ownership convention,
// granting admins access to any repo without changing rest-server itself.
func effectiveUser(who *apitype.WhoIsResponse, admin, privateRepos bool, urlPath string) string {
	if privateRepos && admin {
		if repo := firstPathSegment(urlPath); repo != "" {
			return repo
		}
	}
	return resolveUser(who)
}

// accessCapability is the fixed Tailscale grant capability that identifies a
// restic-gw client. Whether holding it is required to use the server at all
// is controlled by -require-capability; a grant with "admin": true always
// allows the peer to access any repository under -private-repos, not just
// the one matching its Tailscale identity, regardless of that flag. Example
// ACL grants, one rule per group so each peer gets exactly one JSON value
// for this capability:
//
//	"grants": [
//	    {"src": ["group:restic-users"], "dst": ["tag:restic-gw"], "app": {
//	        "restic.net/cap/access": [{}],
//	    }},
//	    {"src": ["group:restic-admins"], "dst": ["tag:restic-gw"], "app": {
//	        "restic.net/cap/access": [{"admin": true}],
//	    }},
//	],
const accessCapability = tailcfg.PeerCapability("restic.net/cap/access")

type accessGrant struct {
	Admin bool `json:"admin"`
}

// checkAccess reports whether the peer is granted access -- either because
// it holds the access capability, or because required is false -- and
// whether any of its access-capability grants set admin.
//
// Each grant value is parsed independently, rather than via
// tailcfg.UnmarshalCapJSON, so that one malformed value (e.g. an ACL typo)
// can't discard a valid "admin": true entry alongside it.
func checkAccess(capMap tailcfg.PeerCapMap, required bool) (granted, admin bool) {
	raws, ok := capMap[accessCapability]
	if !ok {
		return !required, false
	}
	for _, raw := range raws {
		var g accessGrant
		if err := json.Unmarshal([]byte(raw), &g); err != nil {
			log.Printf("failed to parse %q grant: %v", accessCapability, err)
			continue
		}
		if g.Admin {
			return true, true
		}
	}
	return true, false
}

// firstPathSegment returns the first "/"-delimited component of an absolute
// URL path, e.g. "/alice/config" -> "alice".
func firstPathSegment(urlPath string) string {
	segment, _, _ := strings.Cut(strings.TrimPrefix(urlPath, "/"), "/")
	return segment
}
