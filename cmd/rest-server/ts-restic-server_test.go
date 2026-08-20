package main

import (
	"testing"

	"tailscale.com/tailcfg"
)

func TestHasCapability(t *testing.T) {
	const capability = "example.com/restic-backup"

	tests := []struct {
		name    string
		capMap  tailcfg.PeerCapMap
		require string
		want    bool
	}{
		{name: "not required", require: "", want: true},
		{name: "granted", capMap: tailcfg.PeerCapMap{tailcfg.PeerCapability(capability): {}}, require: capability, want: true},
		{name: "not granted", require: capability, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := hasCapability(test.capMap, test.require); got != test.want {
				t.Fatalf("hasCapability() = %t, want %t", got, test.want)
			}
		})
	}
}
