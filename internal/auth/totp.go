package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // HMAC-SHA1 is mandated by the TOTP/RFC 6238 standard
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	totpPeriod = 30
	totpDigits = 6
	totpIssuer = "VPN"
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateTOTPSecret returns a new base32 TOTP secret (160-bit).
func GenerateTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random: %w", err)
	}
	return b32.EncodeToString(b), nil
}

// TOTPURI builds an otpauth:// URI for authenticator apps.
func TOTPURI(secret, account string) string {
	label := url.PathEscape(totpIssuer + ":" + account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", totpIssuer)
	q.Set("period", "30")
	q.Set("digits", "6")
	q.Set("algorithm", "SHA1")
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// totpAt computes the 6-digit code for a secret at a unix time.
func totpAt(secret string, t time.Time) (string, error) {
	key, err := b32.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}
	counter := uint64(t.Unix()) / totpPeriod
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	code := (uint32(sum[offset]&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])) % 1000000
	return fmt.Sprintf("%06d", code), nil
}

// ValidateTOTP reports whether code matches the secret within ±1 time step.
func ValidateTOTP(secret, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return false
	}
	now := time.Now()
	for _, skew := range []time.Duration{-totpPeriod * time.Second, 0, totpPeriod * time.Second} {
		if c, err := totpAt(secret, now.Add(skew)); err == nil && hmac.Equal([]byte(c), []byte(code)) {
			return true
		}
	}
	return false
}
