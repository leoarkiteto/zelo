package security

import "testing"

func TestCSRFTokenValidation(t *testing.T) {
	token := "valid-csrf-token"
	if !ValidateCSRF(token, token) {
		t.Fatal("ValidateCSRF() = false, want true for matching token")
	}
	if ValidateCSRF("forged-token", token) {
		t.Fatal("ValidateCSRF() = true, want false for mismatched token")
	}
}

func TestRandomTokensAreUnique(t *testing.T) {
	a, err := randomToken()
	if err != nil {
		t.Fatalf("randomToken() error = %v", err)
	}
	b, err := randomToken()
	if err != nil {
		t.Fatalf("randomToken() error = %v", err)
	}
	if a == b {
		t.Fatal("randomToken() returned duplicate values")
	}
}
