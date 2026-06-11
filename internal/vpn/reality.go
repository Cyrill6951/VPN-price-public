package vpn

import (
	"fmt"
	"net/url"
)

// realityParams holds everything needed to render a VLESS+Reality URI.
type realityParams struct {
	UUID      string
	Host      string
	Port      int
	SNI       string
	PublicKey string // server Reality public key (pbk)
	ShortID   string
	Label     string
}

// renderRealityURI builds a vless:// import URI for the Xray/v2ray ecosystem.
func renderRealityURI(p realityParams) string {
	q := url.Values{}
	q.Set("type", "tcp")
	q.Set("security", "reality")
	q.Set("encryption", "none")
	q.Set("flow", "xtls-rprx-vision")
	q.Set("sni", p.SNI)
	q.Set("fp", "chrome")
	q.Set("pbk", p.PublicKey)
	q.Set("sid", p.ShortID)

	u := url.URL{
		Scheme:   "vless",
		User:     url.User(p.UUID),
		Host:     fmt.Sprintf("%s:%d", p.Host, p.Port),
		RawQuery: q.Encode(),
		Fragment: p.Label,
	}
	return u.String()
}
