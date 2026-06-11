package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIssueAndParseAccess(t *testing.T) {
	tm := NewTokenManager("secret", 15*time.Minute, time.Hour)
	uid := uuid.New()

	token, jti, exp, err := tm.IssueAccess(uid, RoleAdmin)
	if err != nil {
		t.Fatalf("IssueAccess: %v", err)
	}
	if jti == "" || time.Until(exp) <= 0 {
		t.Fatal("expected non-empty jti and future expiry")
	}

	claims, err := tm.ParseAccess(token)
	if err != nil {
		t.Fatalf("ParseAccess: %v", err)
	}
	if claims.Subject != uid.String() {
		t.Errorf("subject = %q, want %q", claims.Subject, uid.String())
	}
	if claims.Role != RoleAdmin {
		t.Errorf("role = %q, want %q", claims.Role, RoleAdmin)
	}
	if claims.ID != jti {
		t.Errorf("jti = %q, want %q", claims.ID, jti)
	}
}

func TestParseAccess_Expired(t *testing.T) {
	tm := NewTokenManager("secret", -time.Minute, time.Hour)
	token, _, _, err := tm.IssueAccess(uuid.New(), RoleUser)
	if err != nil {
		t.Fatalf("IssueAccess: %v", err)
	}
	if _, err := tm.ParseAccess(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestParseAccess_WrongSecret(t *testing.T) {
	issuer := NewTokenManager("secret-a", time.Minute, time.Hour)
	verifier := NewTokenManager("secret-b", time.Minute, time.Hour)
	token, _, _, _ := issuer.IssueAccess(uuid.New(), RoleUser)
	if _, err := verifier.ParseAccess(token); err == nil {
		t.Fatal("expected token signed with another secret to be rejected")
	}
}

func TestRefreshTokenHashStable(t *testing.T) {
	tm := NewTokenManager("secret", time.Minute, time.Hour)
	token, hash, err := tm.NewRefreshToken()
	if err != nil {
		t.Fatalf("NewRefreshToken: %v", err)
	}
	rehash := HashRefreshToken(token)
	if string(hash) != string(rehash) {
		t.Error("HashRefreshToken must be deterministic for the same token")
	}
}
