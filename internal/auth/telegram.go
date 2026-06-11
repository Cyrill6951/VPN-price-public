package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TelegramAuth holds the fields returned by the Telegram Login widget.
type TelegramAuth struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
	AuthDate  int64  `json:"auth_date"`
	Hash      string `json:"hash"`
}

// maxTelegramAuthAge rejects stale login payloads (replay protection).
const maxTelegramAuthAge = 24 * time.Hour

// VerifyTelegramAuth validates the Telegram Login signature per
// https://core.telegram.org/widgets/login#checking-authorization.
func VerifyTelegramAuth(botToken string, a TelegramAuth) error {
	if botToken == "" {
		return fmt.Errorf("telegram login not configured")
	}
	if a.Hash == "" {
		return ErrTelegramSignature
	}
	if time.Since(time.Unix(a.AuthDate, 0)) > maxTelegramAuthAge {
		return fmt.Errorf("telegram auth data expired")
	}

	pairs := map[string]string{
		"id":        strconv.FormatInt(a.ID, 10),
		"auth_date": strconv.FormatInt(a.AuthDate, 10),
	}
	if a.FirstName != "" {
		pairs["first_name"] = a.FirstName
	}
	if a.LastName != "" {
		pairs["last_name"] = a.LastName
	}
	if a.Username != "" {
		pairs["username"] = a.Username
	}
	if a.PhotoURL != "" {
		pairs["photo_url"] = a.PhotoURL
	}

	keys := make([]string, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(pairs[k])
	}

	secret := sha256.Sum256([]byte(botToken))
	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte(sb.String()))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(a.Hash))) {
		return ErrTelegramSignature
	}
	return nil
}
