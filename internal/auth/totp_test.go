package auth

import (
	"testing"
	"time"
)

func TestTOTPGenerateAndValidate(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	code, err := totpAt(secret, time.Now())
	if err != nil {
		t.Fatalf("totpAt: %v", err)
	}
	if len(code) != totpDigits {
		t.Fatalf("code length = %d, want %d", len(code), totpDigits)
	}
	if !ValidateTOTP(secret, code) {
		t.Error("ValidateTOTP should accept the current code")
	}
	if ValidateTOTP(secret, "000000") && code != "000000" {
		t.Error("ValidateTOTP should reject a wrong code")
	}
}

func TestTOTPURI(t *testing.T) {
	uri := TOTPURI("ABCDEFGH", "alice@example.com")
	if uri == "" || uri[:10] != "otpauth://" {
		t.Errorf("unexpected otpauth uri: %q", uri)
	}
}
