package vpn

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
)

// wgParams holds everything needed to render a WireGuard client config.
type wgParams struct {
	ClientPrivateKey string
	ClientAddress    string // CIDR host, e.g. 10.7.0.2/32
	DNS              string
	ServerPublicKey  string
	PresharedKey     string
	Endpoint         string // host:port
}

// renderWireGuardConfig produces a standard WireGuard .conf for the client.
func renderWireGuardConfig(p wgParams) string {
	var b strings.Builder
	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", p.ClientPrivateKey)
	fmt.Fprintf(&b, "Address = %s\n", p.ClientAddress)
	if p.DNS != "" {
		fmt.Fprintf(&b, "DNS = %s\n", p.DNS)
	}
	b.WriteString("\n[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", p.ServerPublicKey)
	if p.PresharedKey != "" {
		fmt.Fprintf(&b, "PresharedKey = %s\n", p.PresharedKey)
	}
	b.WriteString("AllowedIPs = 0.0.0.0/0, ::/0\n")
	fmt.Fprintf(&b, "Endpoint = %s\n", p.Endpoint)
	b.WriteString("PersistentKeepalive = 25\n")
	return b.String()
}

// allocateWireGuardIP returns the lowest free host address in an IPv4 subnet,
// skipping the network address and the server gateway (.1). used contains the
// already-assigned client addresses (with or without a /mask suffix).
func allocateWireGuardIP(subnet string, used []string) (string, error) {
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return "", fmt.Errorf("parse subnet %q: %w", subnet, err)
	}
	base := ipNet.IP.To4()
	if base == nil {
		return "", fmt.Errorf("only IPv4 subnets are supported, got %q", subnet)
	}

	taken := make(map[uint32]struct{}, len(used))
	for _, u := range used {
		host := u
		if i := strings.IndexByte(host, '/'); i >= 0 {
			host = host[:i]
		}
		if ip := net.ParseIP(strings.TrimSpace(host)).To4(); ip != nil {
			taken[binary.BigEndian.Uint32(ip)] = struct{}{}
		}
	}

	ones, bits := ipNet.Mask.Size()
	network := binary.BigEndian.Uint32(base)
	size := uint32(1) << uint(bits-ones)
	// Skip network (offset 0) and gateway (offset 1); stop before broadcast.
	for offset := uint32(2); offset < size-1; offset++ {
		candidate := network + offset
		if _, ok := taken[candidate]; ok {
			continue
		}
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, candidate)
		return ip.String() + "/32", nil
	}
	return "", ErrSubnetExhausted
}
