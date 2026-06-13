package vpn

import (
	"fmt"
	"net/url"
)

// trojanParams holds everything needed to render a Trojan+Reality URI.
type trojanParams struct {
	Password  string
	Host      string
	Port      int
	SNI       string
	PublicKey string // Reality public key (pbk)
	ShortID   string
	Label     string
}

// renderTrojanURI builds a trojan:// URI secured with Reality.
func renderTrojanURI(p trojanParams) string {
	q := url.Values{}
	q.Set("type", "tcp")
	q.Set("security", "reality")
	q.Set("flow", "xtls-rprx-vision")
	q.Set("sni", p.SNI)
	q.Set("fp", "chrome")
	q.Set("pbk", p.PublicKey)
	q.Set("sid", p.ShortID)

	u := url.URL{
		Scheme:   "trojan",
		User:     url.User(p.Password),
		Host:     fmt.Sprintf("%s:%d", p.Host, p.Port),
		RawQuery: q.Encode(),
		Fragment: p.Label,
	}
	return u.String()
}
