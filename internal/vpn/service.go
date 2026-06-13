package vpn

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/vpnsaas/platform/internal/platform/storage"
)

// Service orchestrates VPN config generation, storage, provisioning and persistence.
type Service struct {
	repo        *Repository
	store       *storage.Store
	cipher      *Cipher
	provisioner Provisioner
	log         *slog.Logger
}

// NewService wires the VPN service.
func NewService(repo *Repository, store *storage.Store, cipher *Cipher, prov Provisioner, log *slog.Logger) *Service {
	return &Service{repo: repo, store: store, cipher: cipher, provisioner: prov, log: log}
}

// CreateInput is the request to provision a new VPN for a user.
type CreateInput struct {
	UserID    uuid.UUID
	CountryID uuid.UUID
	Protocol  Protocol
	PlanID    *uuid.UUID
	Label     *string
	// Routing selects full tunnel (default) or split (RU resources direct).
	// Applies to WireGuard only.
	Routing RoutingMode
}

// Create selects a server, generates the config, provisions the node and persists everything.
func (s *Service) Create(ctx context.Context, in CreateInput) (VPNView, error) {
	if !in.Protocol.Valid() {
		return VPNView{}, ErrUnsupportedProtocol
	}

	days, maxDevices := 30, 1
	if in.PlanID != nil {
		plan, err := s.repo.GetPlan(ctx, *in.PlanID)
		if err != nil {
			return VPNView{}, err
		}
		days, maxDevices = plan.Days, plan.MaxDevices
	}

	srv, err := s.repo.SelectServer(ctx, in.CountryID, in.Protocol)
	if err != nil {
		return VPNView{}, err
	}

	configID := uuid.New()
	label := "VPN"
	if in.Label != nil && *in.Label != "" {
		label = *in.Label
	}

	gen, err := s.generate(ctx, srv, in.Protocol, label, in.Routing)
	if err != nil {
		return VPNView{}, err
	}

	// Provision the node before persisting so a node failure aborts the create.
	if err := s.provision(ctx, srv, in.Protocol, gen); err != nil {
		return VPNView{}, fmt.Errorf("provision node: %w", err)
	}

	cfgKey, qrKey, err := s.storeConfig(ctx, configID, gen)
	if err != nil {
		return VPNView{}, err
	}

	params := CreateParams{
		UserID:          in.UserID,
		Server:          srv,
		CountryID:       in.CountryID,
		PlanID:          in.PlanID,
		Protocol:        in.Protocol,
		Label:           in.Label,
		MaxDevices:      maxDevices,
		ExpiresAt:       time.Now().Add(time.Duration(days) * 24 * time.Hour),
		URI:             gen.uri,
		ConfigObjectKey: cfgKey,
		QRObjectKey:     qrKey,
		Hash:            gen.hash,
		PublicKey:       gen.publicKey,
		ClientUUID:      gen.clientUUID,
		AssignedIP:      gen.assignedIP,
	}
	if err := s.applyEncryptedKeys(gen, &params); err != nil {
		return VPNView{}, err
	}

	sub, cfg, err := s.repo.Persist(ctx, params, configID)
	if err != nil {
		return VPNView{}, err
	}
	return VPNView{Subscription: sub, Config: cfg, Host: srv.PublicHost}, nil
}

// generate produces protocol-specific material and the rendered config.
func (s *Service) generate(ctx context.Context, srv Server, protocol Protocol, label string, routing RoutingMode) (generated, error) {
	switch protocol {
	case ProtocolWireGuard:
		return s.generateWireGuard(ctx, srv, routing)
	case ProtocolVLESSReality:
		return s.generateReality(srv, label)
	case ProtocolShadowsocks:
		return s.generateShadowsocks(srv, label)
	default:
		return generated{}, ErrUnsupportedProtocol
	}
}

func (s *Service) generateShadowsocks(srv Server, label string) (generated, error) {
	if srv.SSPort == nil || srv.SSMethod == nil || srv.SSServerKey == nil {
		return generated{}, ErrServerNotConfigured
	}
	userKey, err := GenerateShadowsocksKey()
	if err != nil {
		return generated{}, err
	}
	email := NewUUID()
	uri := renderShadowsocksURI(ssParams{
		Method:    *srv.SSMethod,
		ServerKey: *srv.SSServerKey,
		UserKey:   userKey,
		Host:      srv.PublicHost,
		Port:      *srv.SSPort,
		Label:     label,
	})
	return generated{
		uri:        uri,
		configText: uri,
		hash:       sha256hex(uri),
		clientUUID: &email,
		ssUserKey:  &userKey,
	}, nil
}

func (s *Service) generateWireGuard(ctx context.Context, srv Server, routing RoutingMode) (generated, error) {
	if srv.WGPublicKey == nil || srv.WGSubnet == nil || srv.WGPort == nil {
		return generated{}, ErrServerNotConfigured
	}
	priv, pub, err := GenerateWireGuardKeypair()
	if err != nil {
		return generated{}, err
	}
	psk, err := GeneratePresharedKey()
	if err != nil {
		return generated{}, err
	}
	used, err := s.repo.ListUsedIPs(ctx, srv.ID)
	if err != nil {
		return generated{}, err
	}
	ip, err := allocateWireGuardIP(*srv.WGSubnet, used)
	if err != nil {
		return generated{}, err
	}
	dns := "1.1.1.1"
	if srv.WGDNS != nil {
		dns = *srv.WGDNS
	}
	conf := renderWireGuardConfig(wgParams{
		ClientPrivateKey: priv,
		ClientAddress:    ip,
		DNS:              dns,
		ServerPublicKey:  *srv.WGPublicKey,
		PresharedKey:     psk,
		Endpoint:         fmt.Sprintf("%s:%d", srv.PublicHost, *srv.WGPort),
		AllowedIPs:       allowedIPsFor(routing),
	})
	ipHost := strings.TrimSuffix(ip, "/32")
	return generated{
		uri:        "",
		configText: conf,
		hash:       sha256hex(conf),
		privateKey: &priv,
		publicKey:  &pub,
		psk:        &psk,
		assignedIP: &ipHost,
	}, nil
}

func (s *Service) generateReality(srv Server, label string) (generated, error) {
	if srv.RealityPublicKey == nil || srv.RealityPort == nil || srv.RealitySNI == nil {
		return generated{}, ErrServerNotConfigured
	}
	shortID := ""
	if srv.RealityShortID != nil {
		shortID = *srv.RealityShortID
	}
	clientUUID := NewUUID()
	uri := renderRealityURI(realityParams{
		UUID:      clientUUID,
		Host:      srv.PublicHost,
		Port:      *srv.RealityPort,
		SNI:       *srv.RealitySNI,
		PublicKey: *srv.RealityPublicKey,
		ShortID:   shortID,
		Label:     label,
	})
	return generated{
		uri:        uri,
		configText: uri,
		hash:       sha256hex(uri),
		clientUUID: &clientUUID,
	}, nil
}

func (s *Service) provision(ctx context.Context, srv Server, protocol Protocol, gen generated) error {
	switch protocol {
	case ProtocolWireGuard:
		allowed := ""
		if gen.assignedIP != nil {
			allowed = *gen.assignedIP + "/32"
		}
		psk := ""
		if gen.psk != nil {
			psk = *gen.psk
		}
		return s.provisioner.AddWireGuardPeer(ctx, srv, derefStr(gen.publicKey), psk, allowed)
	case ProtocolVLESSReality:
		return s.provisioner.AddVLESSClient(ctx, srv, derefStr(gen.clientUUID))
	case ProtocolShadowsocks:
		return s.provisioner.AddShadowsocksClient(ctx, srv, derefStr(gen.ssUserKey), derefStr(gen.clientUUID))
	default:
		return ErrUnsupportedProtocol
	}
}

// List returns a user's VPNs.
func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]VPNView, error) {
	return s.repo.ListUserVPNs(ctx, userID)
}

// FetchConfig returns the decrypted client config text and its content type.
func (s *Service) FetchConfig(ctx context.Context, userID, subscriptionID uuid.UUID) ([]byte, string, error) {
	art, err := s.repo.GetArtifactForUser(ctx, userID, subscriptionID)
	if err != nil {
		return nil, "", err
	}
	enc, err := s.store.Get(ctx, art.ConfigObjectKey)
	if err != nil {
		return nil, "", err
	}
	plain, err := s.cipher.Decrypt(string(enc))
	if err != nil {
		return nil, "", err
	}
	return plain, "text/plain; charset=utf-8", nil
}

// FetchQR returns the PNG QR code for a user's subscription config.
func (s *Service) FetchQR(ctx context.Context, userID, subscriptionID uuid.UUID) ([]byte, error) {
	art, err := s.repo.GetArtifactForUser(ctx, userID, subscriptionID)
	if err != nil {
		return nil, err
	}
	if art.QRObjectKey == "" {
		return nil, ErrNotFound // e.g. split-tunnel config too large for a QR
	}
	return s.store.Get(ctx, art.QRObjectKey)
}

// Delete revokes a VPN: deprovisions the node and purges stored artifacts.
func (s *Service) Delete(ctx context.Context, userID, subscriptionID uuid.UUID) error {
	del, err := s.repo.SoftDelete(ctx, userID, subscriptionID)
	if err != nil {
		return err
	}
	switch del.Protocol {
	case ProtocolWireGuard:
		if del.PublicKey != nil {
			if err := s.provisioner.RemoveWireGuardPeer(ctx, del.Server, *del.PublicKey); err != nil {
				s.log.Warn("deprovision wireguard peer failed", "error", err)
			}
		}
	case ProtocolVLESSReality:
		if del.ClientUUID != nil {
			if err := s.provisioner.RemoveVLESSClient(ctx, del.Server, *del.ClientUUID); err != nil {
				s.log.Warn("deprovision vless client failed", "error", err)
			}
		}
	case ProtocolShadowsocks:
		if del.ClientUUID != nil {
			if err := s.provisioner.RemoveShadowsocksClient(ctx, del.Server, *del.ClientUUID); err != nil {
				s.log.Warn("deprovision shadowsocks client failed", "error", err)
			}
		}
	}
	for _, key := range del.ObjectKeys {
		if err := s.store.Remove(ctx, key); err != nil {
			s.log.Warn("remove object failed", "key", key, "error", err)
		}
	}
	return nil
}

// Countries returns the catalogue of enabled countries.
func (s *Service) Countries(ctx context.Context) ([]CountryRef, error) {
	return s.repo.ListCountries(ctx)
}

// Plans returns the catalogue of active plans.
func (s *Service) Plans(ctx context.Context) ([]PlanRef, error) {
	return s.repo.ListPlans(ctx)
}

// storeConfig encrypts and stores the config (and a best-effort QR) in object
// storage, returning their keys.
func (s *Service) storeConfig(ctx context.Context, configID uuid.UUID, gen generated) (cfgKey, qrKey string, err error) {
	cfgKey = "cfg/" + configID.String()
	encConfig, err := s.cipher.Encrypt([]byte(gen.configText))
	if err != nil {
		return "", "", err
	}
	if err := s.store.Put(ctx, cfgKey, []byte(encConfig), "application/octet-stream"); err != nil {
		return "", "", err
	}
	qrContent := gen.uri
	if qrContent == "" {
		qrContent = gen.configText
	}
	if png, qrErr := renderQRPNG(qrContent, 512); qrErr == nil {
		qrKey = "qr/" + configID.String() + ".png"
		if err := s.store.Put(ctx, qrKey, png, "image/png"); err != nil {
			return "", "", err
		}
	} else {
		s.log.Info("skipping QR (config too large)", "bytes", len(qrContent))
	}
	return cfgKey, qrKey, nil
}

// applyEncryptedKeys encrypts the generated secrets into the persistence params.
func (s *Service) applyEncryptedKeys(gen generated, params *CreateParams) error {
	enc := func(p *string) (*string, error) {
		if p == nil {
			return nil, nil
		}
		v, err := s.cipher.Encrypt([]byte(*p))
		if err != nil {
			return nil, err
		}
		return &v, nil
	}
	var err error
	if params.PrivateKeyEnc, err = enc(gen.privateKey); err != nil {
		return err
	}
	if params.PSKEnc, err = enc(gen.psk); err != nil {
		return err
	}
	if gen.ssUserKey != nil {
		if params.PrivateKeyEnc, err = enc(gen.ssUserKey); err != nil {
			return err
		}
	}
	return nil
}

// MigrationResult describes one migrated subscription.
type MigrationResult struct {
	UserID     uuid.UUID
	TelegramID *int64
	Country    string
	SubID      uuid.UUID
}

// MigrateServer re-issues every active subscription on a failed/blocked server
// onto a reserve server in the same country, deprovisioning the old node
// best-effort. Returns the migrated subscriptions (for notification).
func (s *Service) MigrateServer(ctx context.Context, fromServerID uuid.UUID, reason string) ([]MigrationResult, error) {
	subs, err := s.repo.ActiveSubsOnServer(ctx, fromServerID)
	if err != nil {
		return nil, err
	}
	oldServer, _ := s.repo.GetServerByID(ctx, fromServerID)

	results := make([]MigrationResult, 0, len(subs))
	for _, sub := range subs {
		reserve, err := s.repo.SelectReserveServer(ctx, sub.CountryID, sub.Protocol, fromServerID)
		if err != nil {
			s.log.Warn("no reserve server for migration", "sub", sub.SubID, "country", sub.Country)
			continue
		}
		oldPub, oldUUID, _ := s.repo.OldKeyForSub(ctx, sub.SubID)

		label := "VPN"
		if sub.Label != nil && *sub.Label != "" {
			label = *sub.Label
		}
		gen, err := s.generate(ctx, reserve, sub.Protocol, label, RoutingFull)
		if err != nil {
			s.log.Warn("migrate: generate failed", "sub", sub.SubID, "error", err)
			continue
		}
		if err := s.provision(ctx, reserve, sub.Protocol, gen); err != nil {
			s.log.Warn("migrate: provision failed", "sub", sub.SubID, "error", err)
			continue
		}

		configID := uuid.New()
		cfgKey, qrKey, err := s.storeConfig(ctx, configID, gen)
		if err != nil {
			s.log.Warn("migrate: store failed", "sub", sub.SubID, "error", err)
			continue
		}
		params := CreateParams{
			UserID: sub.UserID, Server: reserve, Protocol: sub.Protocol,
			URI: gen.uri, ConfigObjectKey: cfgKey, QRObjectKey: qrKey, Hash: gen.hash,
			PublicKey: gen.publicKey, ClientUUID: gen.clientUUID, AssignedIP: gen.assignedIP,
		}
		if err := s.applyEncryptedKeys(gen, &params); err != nil {
			s.log.Warn("migrate: encrypt failed", "sub", sub.SubID, "error", err)
			continue
		}
		if err := s.repo.Reassign(ctx, params, configID, sub.SubID, fromServerID, reason); err != nil {
			s.log.Warn("migrate: reassign failed", "sub", sub.SubID, "error", err)
			continue
		}

		s.deprovisionOld(ctx, oldServer, sub.Protocol, oldPub, oldUUID)
		results = append(results, MigrationResult{UserID: sub.UserID, TelegramID: sub.TelegramID, Country: sub.Country, SubID: sub.SubID})
	}
	if len(results) > 0 {
		s.log.Info("migration complete", "from", fromServerID, "migrated", len(results), "reason", reason)
	}
	return results, nil
}

func (s *Service) deprovisionOld(ctx context.Context, oldServer Server, protocol Protocol, pub, clientUUID *string) {
	if oldServer.ID == uuid.Nil {
		return
	}
	switch protocol {
	case ProtocolWireGuard:
		if pub != nil {
			_ = s.provisioner.RemoveWireGuardPeer(ctx, oldServer, *pub)
		}
	case ProtocolVLESSReality:
		if clientUUID != nil {
			_ = s.provisioner.RemoveVLESSClient(ctx, oldServer, *clientUUID)
		}
	case ProtocolShadowsocks:
		if clientUUID != nil {
			_ = s.provisioner.RemoveShadowsocksClient(ctx, oldServer, *clientUUID)
		}
	}
}

func sha256hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
