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

	cfgKey := "cfg/" + configID.String()

	encConfig, err := s.cipher.Encrypt([]byte(gen.configText))
	if err != nil {
		return VPNView{}, err
	}
	if err := s.store.Put(ctx, cfgKey, []byte(encConfig), "application/octet-stream"); err != nil {
		return VPNView{}, err
	}

	// QR is best-effort: split-tunnel WireGuard configs exceed the QR capacity,
	// so large configs are delivered as a file instead of failing the purchase.
	qrKey := ""
	qrContent := gen.uri
	if qrContent == "" {
		qrContent = gen.configText
	}
	if png, qrErr := renderQRPNG(qrContent, 512); qrErr == nil {
		qrKey = "qr/" + configID.String() + ".png"
		if err := s.store.Put(ctx, qrKey, png, "image/png"); err != nil {
			return VPNView{}, err
		}
	} else {
		s.log.Info("skipping QR (config too large)", "bytes", len(qrContent))
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
	if gen.privateKey != nil {
		enc, encErr := s.cipher.Encrypt([]byte(*gen.privateKey))
		if encErr != nil {
			return VPNView{}, encErr
		}
		params.PrivateKeyEnc = &enc
	}
	if gen.psk != nil {
		enc, encErr := s.cipher.Encrypt([]byte(*gen.psk))
		if encErr != nil {
			return VPNView{}, encErr
		}
		params.PSKEnc = &enc
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
	default:
		return generated{}, ErrUnsupportedProtocol
	}
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
