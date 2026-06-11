package auth

import (
	"errors"
	"fmt"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const minPasswordLen = 12

// ValidatePassword enforces the password policy from docs/13_Security.md:
// at least 12 characters with upper, lower, digit and special classes.
func ValidatePassword(pw string) error {
	if len(pw) < minPasswordLen {
		return fmt.Errorf("password must be at least %d characters", minPasswordLen)
	}
	var upper, lower, digit, special bool
	for _, r := range pw {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsLower(r):
			lower = true
		case unicode.IsDigit(r):
			digit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			special = true
		}
	}
	if !upper || !lower || !digit || !special {
		return errors.New("password must include upper, lower, digit and special characters")
	}
	return nil
}

// HashPassword returns a bcrypt hash of the given plaintext password.
func HashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(h), nil
}

// CheckPassword reports whether plaintext matches the stored bcrypt hash.
func CheckPassword(hash, plaintext string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)) == nil
}
