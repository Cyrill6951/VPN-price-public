// Package vpn generates and manages VPN configurations (WireGuard, VLESS+Reality),
// stores them in object storage and provisions peers on VPN nodes.
package vpn

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Protocol identifies a supported VPN protocol.
type Protocol string

const (
	ProtocolWireGuard    Protocol = "wireguard"
	ProtocolVLESSReality Protocol = "vless_reality"
)

// Valid reports whether p is a supported protocol.
func (p Protocol) Valid() bool {
	return p == ProtocolWireGuard || p == ProtocolVLESSReality
}

// Server is a VPN node with its protocol parameters.
type Server struct {
	ID         uuid.UUID
	CountryID  uuid.UUID
	PublicHost string
	AgentURL   *string
	Status     string

	WGPort      *int
	WGPublicKey *string
	WGSubnet    *string
	WGDNS       *string

	RealityPort      *int
	RealityPublicKey *string
	RealitySNI       *string
	RealityShortID   *string
	RealityDest      *string
}

// Plan is a purchasable tariff.
type Plan struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Days       int       `json:"days"`
	MaxDevices int       `json:"max_devices"`
}

// Subscription links a user to a server/protocol with an expiry.
type Subscription struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	ServerID  uuid.UUID `json:"server_id"`
	CountryID uuid.UUID `json:"country_id"`
	Protocol  Protocol  `json:"protocol"`
	Status    string    `json:"status"`
	Label     *string   `json:"label,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// Config is the rendered, stored client configuration metadata.
type Config struct {
	ID        uuid.UUID `json:"id"`
	Protocol  Protocol  `json:"protocol"`
	URI       string    `json:"uri"`
	Hash      string    `json:"hash"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
}

// VPNView is a subscription joined with its current config (list/create result).
type VPNView struct {
	Subscription Subscription `json:"subscription"`
	Config       Config       `json:"config"`
	Country      string       `json:"country"`
	Host         string       `json:"host"`
}

// generated holds the freshly produced material before persistence.
type generated struct {
	uri        string
	configText string
	hash       string

	privateKey *string // WireGuard client private (plaintext, encrypted before store)
	publicKey  *string // WireGuard client public
	psk        *string // preshared key (plaintext)
	clientUUID *string // VLESS client uuid
	assignedIP *string // WireGuard client address
}

// Domain errors.
var (
	ErrUnsupportedProtocol = errors.New("unsupported protocol")
	ErrServerNotConfigured = errors.New("server is not configured for this protocol")
	ErrNoServerAvailable   = errors.New("no server available for the requested country")
	ErrSubnetExhausted     = errors.New("server address pool exhausted")
	ErrNotFound            = errors.New("not found")
)
