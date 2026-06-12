package vpn

import (
	"encoding/base64"
	"errors"
	"net"
	"net/url"
	"strings"
	"testing"
)

func allowedContains(t *testing.T, allowed, ipStr string) bool {
	t.Helper()
	ip := net.ParseIP(ipStr)
	for _, c := range strings.Split(allowed, ", ") {
		_, n, err := net.ParseCIDR(strings.TrimSpace(c))
		if err == nil && n.Contains(ip) {
			return true
		}
	}
	return false
}

func TestSplitRouting_RU(t *testing.T) {
	allowed := allowedIPsFor(RoutingSplitRU)
	if allowed == "" || allowed == fullAllowedIPs {
		t.Fatal("split routing produced no exclusions")
	}
	// A Russian address (inside 2.56.24.0/22) must NOT be tunneled.
	if allowedContains(t, allowed, "2.56.24.5") {
		t.Error("Russian IP should be excluded from the tunnel (route direct)")
	}
	// A private address must NOT be tunneled.
	if allowedContains(t, allowed, "10.1.2.3") {
		t.Error("private IP should be excluded from the tunnel")
	}
	// A non-Russian public address (Google DNS) MUST be tunneled.
	if !allowedContains(t, allowed, "8.8.8.8") {
		t.Error("non-Russian IP should be routed through the tunnel")
	}
}

func TestRoutingFull(t *testing.T) {
	if allowedIPsFor(RoutingFull) != fullAllowedIPs {
		t.Errorf("full routing = %q, want %q", allowedIPsFor(RoutingFull), fullAllowedIPs)
	}
}

func TestGenerateWireGuardKeypair(t *testing.T) {
	priv, pub, err := GenerateWireGuardKeypair()
	if err != nil {
		t.Fatalf("GenerateWireGuardKeypair: %v", err)
	}
	for name, k := range map[string]string{"priv": priv, "pub": pub} {
		raw, err := base64.StdEncoding.DecodeString(k)
		if err != nil {
			t.Fatalf("%s not valid std base64: %v", name, err)
		}
		if len(raw) != 32 {
			t.Errorf("%s length = %d, want 32", name, len(raw))
		}
	}
	if priv == pub {
		t.Error("private and public keys must differ")
	}
}

func TestGenerateRealityKeypair(t *testing.T) {
	priv, pub, err := GenerateRealityKeypair()
	if err != nil {
		t.Fatalf("GenerateRealityKeypair: %v", err)
	}
	if _, err := base64.RawURLEncoding.DecodeString(priv); err != nil {
		t.Errorf("priv not valid base64url: %v", err)
	}
	if _, err := base64.RawURLEncoding.DecodeString(pub); err != nil {
		t.Errorf("pub not valid base64url: %v", err)
	}
}

func TestCipherRoundtrip(t *testing.T) {
	key := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	c, err := NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	plaintext := []byte("super-secret-wireguard-private-key")
	enc, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if strings.Contains(enc, string(plaintext)) {
		t.Error("ciphertext leaks plaintext")
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(dec) != string(plaintext) {
		t.Errorf("roundtrip = %q, want %q", dec, plaintext)
	}
}

func TestNewCipher_BadKey(t *testing.T) {
	if _, err := NewCipher("tooshort"); err == nil {
		t.Error("expected error for short key")
	}
}

func TestAllocateWireGuardIP(t *testing.T) {
	// First free host in /24 is .2 (.0 network, .1 gateway).
	ip, err := allocateWireGuardIP("10.7.0.0/24", nil)
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if ip != "10.7.0.2/32" {
		t.Errorf("first ip = %q, want 10.7.0.2/32", ip)
	}

	// Skips already-used addresses.
	ip, err = allocateWireGuardIP("10.7.0.0/24", []string{"10.7.0.2", "10.7.0.3/32"})
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if ip != "10.7.0.4/32" {
		t.Errorf("ip = %q, want 10.7.0.4/32", ip)
	}
}

func TestAllocateWireGuardIP_Exhausted(t *testing.T) {
	// /30 has hosts .1 and .2; we reserve .1 as gateway, leaving only .2.
	if _, err := allocateWireGuardIP("10.0.0.0/30", []string{"10.0.0.2"}); !errors.Is(err, ErrSubnetExhausted) {
		t.Fatalf("err = %v, want ErrSubnetExhausted", err)
	}
}

func TestRenderWireGuardConfig(t *testing.T) {
	conf := renderWireGuardConfig(wgParams{
		ClientPrivateKey: "PRIV",
		ClientAddress:    "10.7.0.2/32",
		DNS:              "1.1.1.1",
		ServerPublicKey:  "SRVPUB",
		PresharedKey:     "PSK",
		Endpoint:         "vpn.example.com:51820",
	})
	for _, want := range []string{
		"[Interface]", "PrivateKey = PRIV", "Address = 10.7.0.2/32",
		"[Peer]", "PublicKey = SRVPUB", "PresharedKey = PSK",
		"Endpoint = vpn.example.com:51820", "AllowedIPs = 0.0.0.0/0, ::/0",
	} {
		if !strings.Contains(conf, want) {
			t.Errorf("config missing %q\n%s", want, conf)
		}
	}
}

func TestRenderRealityURI(t *testing.T) {
	uri := renderRealityURI(realityParams{
		UUID:      "11111111-1111-1111-1111-111111111111",
		Host:      "vpn.example.com",
		Port:      443,
		SNI:       "www.microsoft.com",
		PublicKey: "PBK",
		ShortID:   "abcd1234",
		Label:     "DE-01",
	})
	u, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("parse uri: %v", err)
	}
	if u.Scheme != "vless" {
		t.Errorf("scheme = %q, want vless", u.Scheme)
	}
	if u.User.Username() != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("uuid = %q", u.User.Username())
	}
	q := u.Query()
	if q.Get("security") != "reality" || q.Get("pbk") != "PBK" || q.Get("sid") != "abcd1234" {
		t.Errorf("unexpected query: %v", q)
	}
	if u.Fragment != "DE-01" {
		t.Errorf("fragment = %q, want DE-01", u.Fragment)
	}
}
