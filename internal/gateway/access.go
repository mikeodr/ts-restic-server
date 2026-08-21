package gateway

import (
	"encoding/json"
	"log"

	"tailscale.com/tailcfg"
)

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
