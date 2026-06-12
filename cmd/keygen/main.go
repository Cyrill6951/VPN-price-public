// Command keygen prints a fresh set of VPN node server keys as JSON.
// Used by scripts/add-node to provision a new node and register its public keys.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/vpnsaas/platform/internal/vpn"
)

func main() {
	wgPriv, wgPub, err := vpn.GenerateWireGuardKeypair()
	if err != nil {
		fail(err)
	}
	rPriv, rPub, err := vpn.GenerateRealityKeypair()
	if err != nil {
		fail(err)
	}
	sid, err := vpn.NewShortID()
	if err != nil {
		fail(err)
	}
	ssKey, err := vpn.GenerateShadowsocksKey()
	if err != nil {
		fail(err)
	}

	out := map[string]string{
		"wg_private_key":      wgPriv,
		"wg_public_key":       wgPub,
		"reality_private_key": rPriv,
		"reality_public_key":  rPub,
		"reality_short_id":    sid,
		"ss_server_key":       ssKey,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "keygen error:", err)
	os.Exit(1)
}
