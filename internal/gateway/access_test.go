package gateway

import (
	"testing"

	"tailscale.com/tailcfg"
)

func TestCheckAccess(t *testing.T) {
	tests := []struct {
		name        string
		capMap      tailcfg.PeerCapMap
		required    bool
		wantGranted bool
		wantAdmin   bool
	}{
		{name: "not required, no capability", required: false, wantGranted: true},
		{name: "required, no capability", required: true, wantGranted: false},
		{
			name:        "required, granted, no grant body",
			capMap:      tailcfg.PeerCapMap{accessCapability: {}},
			required:    true,
			wantGranted: true,
		},
		{
			name: "required, granted without admin",
			capMap: tailcfg.PeerCapMap{
				accessCapability: {tailcfg.RawMessage(`{"admin":false}`)},
			},
			required:    true,
			wantGranted: true,
		},
		{
			name: "required, granted with admin",
			capMap: tailcfg.PeerCapMap{
				accessCapability: {tailcfg.RawMessage(`{"admin":true}`)},
			},
			required:    true,
			wantGranted: true,
			wantAdmin:   true,
		},
		{
			name: "not required but admin grant still recognized",
			capMap: tailcfg.PeerCapMap{
				accessCapability: {tailcfg.RawMessage(`{"admin":true}`)},
			},
			required:    false,
			wantGranted: true,
			wantAdmin:   true,
		},
		{
			name: "admin among multiple grants",
			capMap: tailcfg.PeerCapMap{
				accessCapability: {
					tailcfg.RawMessage(`{}`),
					tailcfg.RawMessage(`{"admin":true}`),
				},
			},
			required:    true,
			wantGranted: true,
			wantAdmin:   true,
		},
		{
			name: "malformed grant body still grants but not admin",
			capMap: tailcfg.PeerCapMap{
				accessCapability: {tailcfg.RawMessage(`not json`)},
			},
			required:    true,
			wantGranted: true,
		},
		{
			name: "admin entry survives a malformed sibling entry",
			capMap: tailcfg.PeerCapMap{
				accessCapability: {
					tailcfg.RawMessage(`{"admin":true}`),
					tailcfg.RawMessage(`{"admin":"true"}`), // e.g. ACL typo: string, not bool
				},
			},
			required:    true,
			wantGranted: true,
			wantAdmin:   true,
		},
		{
			name: "malformed entry doesn't grant admin on its own",
			capMap: tailcfg.PeerCapMap{
				accessCapability: {
					tailcfg.RawMessage(`{"admin":"true"}`), // e.g. ACL typo: string, not bool
					tailcfg.RawMessage(`{}`),
				},
			},
			required:    true,
			wantGranted: true,
			wantAdmin:   false,
		},
		{
			name: "unrelated capability doesn't grant required access",
			capMap: tailcfg.PeerCapMap{
				tailcfg.PeerCapability("example.com/other"): {},
			},
			required: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotGranted, gotAdmin := checkAccess(test.capMap, test.required)
			if gotGranted != test.wantGranted || gotAdmin != test.wantAdmin {
				t.Fatalf("checkAccess() = (%t, %t), want (%t, %t)", gotGranted, gotAdmin, test.wantGranted, test.wantAdmin)
			}
		})
	}
}
