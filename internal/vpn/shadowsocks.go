package vpn

import (
	"encoding/base64"
	"fmt"
	"net/url"
)

// ssParams holds everything needed to render a Shadowsocks-2022 URI.
type ssParams struct {
	Method    string
	ServerKey string // server PSK (base64)
	UserKey   string // per-user PSK (base64)
	Host      string
	Port      int
	Label     string
}

// renderShadowsocksURI builds a SIP002 ss:// URI for Shadowsocks-2022 multi-user.
// The userinfo encodes "method:serverPSK:userPSK".
func renderShadowsocksURI(p ssParams) string {
	userinfo := base64.RawURLEncoding.EncodeToString(
		[]byte(p.Method + ":" + p.ServerKey + ":" + p.UserKey))
	u := url.URL{
		Scheme:   "ss",
		User:     url.User(userinfo),
		Host:     fmt.Sprintf("%s:%d", p.Host, p.Port),
		Fragment: p.Label,
	}
	return u.String()
}
