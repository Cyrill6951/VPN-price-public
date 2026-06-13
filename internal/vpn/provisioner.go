package vpn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Provisioner applies (or removes) client configuration on a VPN node.
// Implementations: noopProvisioner (store-only) and agentProvisioner (calls the
// node agent over HTTP).
type Provisioner interface {
	AddWireGuardPeer(ctx context.Context, srv Server, publicKey, presharedKey, allowedIP string) error
	RemoveWireGuardPeer(ctx context.Context, srv Server, publicKey string) error
	AddVLESSClient(ctx context.Context, srv Server, clientUUID string) error
	RemoveVLESSClient(ctx context.Context, srv Server, clientUUID string) error
	AddShadowsocksClient(ctx context.Context, srv Server, userKey, email string) error
	RemoveShadowsocksClient(ctx context.Context, srv Server, email string) error
	AddTrojanClient(ctx context.Context, srv Server, password, email string) error
	RemoveTrojanClient(ctx context.Context, srv Server, email string) error
}

// NewProvisioner returns the provisioner selected by mode ("noop" or "agent").
func NewProvisioner(mode string, log *slog.Logger, agentToken string) Provisioner {
	switch mode {
	case "agent":
		return &agentProvisioner{
			token:  agentToken,
			client: &http.Client{Timeout: 10 * time.Second},
			log:    log,
		}
	default:
		return &noopProvisioner{log: log}
	}
}

// noopProvisioner records intent but does not touch any node. It lets the full
// config-generation flow run without a live VPN node (development/CI).
type noopProvisioner struct{ log *slog.Logger }

func (p *noopProvisioner) AddWireGuardPeer(_ context.Context, srv Server, publicKey, _, allowedIP string) error {
	p.log.Info("noop: add wireguard peer", "server", srv.ID, "pubkey", publicKey, "ip", allowedIP)
	return nil
}
func (p *noopProvisioner) RemoveWireGuardPeer(_ context.Context, srv Server, publicKey string) error {
	p.log.Info("noop: remove wireguard peer", "server", srv.ID, "pubkey", publicKey)
	return nil
}
func (p *noopProvisioner) AddVLESSClient(_ context.Context, srv Server, clientUUID string) error {
	p.log.Info("noop: add vless client", "server", srv.ID, "uuid", clientUUID)
	return nil
}
func (p *noopProvisioner) RemoveVLESSClient(_ context.Context, srv Server, clientUUID string) error {
	p.log.Info("noop: remove vless client", "server", srv.ID, "uuid", clientUUID)
	return nil
}
func (p *noopProvisioner) AddShadowsocksClient(_ context.Context, srv Server, _, email string) error {
	p.log.Info("noop: add shadowsocks client", "server", srv.ID, "email", email)
	return nil
}
func (p *noopProvisioner) RemoveShadowsocksClient(_ context.Context, srv Server, email string) error {
	p.log.Info("noop: remove shadowsocks client", "server", srv.ID, "email", email)
	return nil
}
func (p *noopProvisioner) AddTrojanClient(_ context.Context, srv Server, _, email string) error {
	p.log.Info("noop: add trojan client", "server", srv.ID, "email", email)
	return nil
}
func (p *noopProvisioner) RemoveTrojanClient(_ context.Context, srv Server, email string) error {
	p.log.Info("noop: remove trojan client", "server", srv.ID, "email", email)
	return nil
}

// agentProvisioner talks to the node agent (see cmd/agent) over HTTP.
type agentProvisioner struct {
	token  string
	client *http.Client
	log    *slog.Logger
}

func (p *agentProvisioner) AddWireGuardPeer(ctx context.Context, srv Server, publicKey, presharedKey, allowedIP string) error {
	return p.post(ctx, srv, "/wg/peers", map[string]string{
		"public_key": publicKey, "preshared_key": presharedKey, "allowed_ip": allowedIP,
	})
}
func (p *agentProvisioner) RemoveWireGuardPeer(ctx context.Context, srv Server, publicKey string) error {
	return p.post(ctx, srv, "/wg/peers/remove", map[string]string{"public_key": publicKey})
}
func (p *agentProvisioner) AddVLESSClient(ctx context.Context, srv Server, clientUUID string) error {
	return p.post(ctx, srv, "/vless/clients", map[string]string{"uuid": clientUUID})
}
func (p *agentProvisioner) RemoveVLESSClient(ctx context.Context, srv Server, clientUUID string) error {
	return p.post(ctx, srv, "/vless/clients/remove", map[string]string{"uuid": clientUUID})
}
func (p *agentProvisioner) AddShadowsocksClient(ctx context.Context, srv Server, userKey, email string) error {
	return p.post(ctx, srv, "/ss/clients", map[string]string{"password": userKey, "email": email})
}
func (p *agentProvisioner) RemoveShadowsocksClient(ctx context.Context, srv Server, email string) error {
	return p.post(ctx, srv, "/ss/clients/remove", map[string]string{"email": email})
}
func (p *agentProvisioner) AddTrojanClient(ctx context.Context, srv Server, password, email string) error {
	return p.post(ctx, srv, "/trojan/clients", map[string]string{"password": password, "email": email})
}
func (p *agentProvisioner) RemoveTrojanClient(ctx context.Context, srv Server, email string) error {
	return p.post(ctx, srv, "/trojan/clients/remove", map[string]string{"email": email})
}

func (p *agentProvisioner) post(ctx context.Context, srv Server, path string, payload any) error {
	if srv.AgentURL == nil || *srv.AgentURL == "" {
		return fmt.Errorf("server %s has no agent_url", srv.ID)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, *srv.AgentURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.token)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("call agent: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("agent %s returned %d", path, resp.StatusCode)
	}
	return nil
}
