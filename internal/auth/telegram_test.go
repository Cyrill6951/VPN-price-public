package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"
)

// sign reproduces the Telegram data-check-string signature for testing.
func sign(botToken string, a TelegramAuth) string {
	data := "auth_date=" + strconv.FormatInt(a.AuthDate, 10) +
		"\nfirst_name=" + a.FirstName +
		"\nid=" + strconv.FormatInt(a.ID, 10) +
		"\nusername=" + a.Username
	secret := sha256.Sum256([]byte(botToken))
	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyTelegramAuth(t *testing.T) {
	const bot = "123456:test-bot-token"
	a := TelegramAuth{
		ID:        42,
		FirstName: "Alice",
		Username:  "alice",
		AuthDate:  time.Now().Unix(),
	}
	a.Hash = sign(bot, a)

	if err := VerifyTelegramAuth(bot, a); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}

	tampered := a
	tampered.ID = 43 // hash no longer matches
	if err := VerifyTelegramAuth(bot, tampered); err == nil {
		t.Fatal("tampered payload accepted")
	}

	if err := VerifyTelegramAuth("", a); err == nil {
		t.Fatal("empty bot token must fail")
	}
}

func TestVerifyTelegramAuth_Expired(t *testing.T) {
	const bot = "123456:test-bot-token"
	a := TelegramAuth{
		ID:        42,
		FirstName: "Alice",
		Username:  "alice",
		AuthDate:  time.Now().Add(-48 * time.Hour).Unix(),
	}
	a.Hash = sign(bot, a)
	if err := VerifyTelegramAuth(bot, a); err == nil {
		t.Fatal("stale auth_date must be rejected")
	}
}
