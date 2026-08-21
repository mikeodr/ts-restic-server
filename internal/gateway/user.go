package gateway

import (
	"strings"

	"tailscale.com/client/tailscale/apitype"
)

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

// firstPathSegment returns the first "/"-delimited component of an absolute
// URL path, e.g. "/alice/config" -> "alice".
func firstPathSegment(urlPath string) string {
	segment, _, _ := strings.Cut(strings.TrimPrefix(urlPath, "/"), "/")
	return segment
}
