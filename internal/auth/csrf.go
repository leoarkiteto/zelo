package auth

import (
	"crypto/subtle"
	"fmt"
)

// NewCSRFToken returns a fresh random CSRF token.
func NewCSRFToken() (string, error) {
	tok, err := randomToken()
	if err != nil {
		return "", fmt.Errorf("generate csrf token: %w", err)
	}
	return tok, nil
}

// ValidateCSRF compares a submitted CSRF token against the stored token using
// a constant-time comparison.
func ValidateCSRF(submitted, stored string) bool {
	return subtle.ConstantTimeCompare([]byte(submitted), []byte(stored)) == 1
}
