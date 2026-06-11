package auth

import "testing"

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name    string
		pw      string
		wantErr bool
	}{
		{"too short", "Ab1!xyz", true},
		{"no upper", "abcdefgh1234!", true},
		{"no digit", "Abcdefghij!!", true},
		{"no special", "Abcdefghij12", true},
		{"valid", "Str0ng!Passw0rd", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidatePassword(c.pw)
			if (err != nil) != c.wantErr {
				t.Fatalf("ValidatePassword(%q) err=%v, wantErr=%v", c.pw, err, c.wantErr)
			}
		})
	}
}

func TestHashAndCheckPassword(t *testing.T) {
	const pw = "Str0ng!Passw0rd"
	hash, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !CheckPassword(hash, pw) {
		t.Error("CheckPassword should accept the correct password")
	}
	if CheckPassword(hash, "wrong-Pass1!") {
		t.Error("CheckPassword should reject an incorrect password")
	}
}
