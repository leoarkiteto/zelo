// Package auth implements password hashing, sessions, and CSRF helpers.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Password hashing parameters (OWASP-recommended Argon2id defaults).
const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16

	minPasswordLen = 12
	maxPasswordLen = 256
)

var (
	// ErrPasswordTooShort is returned when a password is shorter than 12 chars.
	ErrPasswordTooShort = errors.New("password must be at least 12 characters")
	// ErrPasswordTooLong is returned when a password is longer than 256 chars.
	ErrPasswordTooLong = errors.New("password must be at most 256 characters")
)

// PasswordHasher hashes and verifies passwords with Argon2id.
type PasswordHasher struct{}

// ValidatePassword checks the password policy.
func (PasswordHasher) ValidatePassword(password string) error {
	if len(password) < minPasswordLen {
		return ErrPasswordTooShort
	}
	if len(password) > maxPasswordLen {
		return ErrPasswordTooLong
	}
	return nil
}

// Hash returns a PHC-formatted Argon2id hash for the password.
func (PasswordHasher) Hash(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// Verify checks a password against a PHC hash using a constant-time compare.
func (PasswordHasher) Verify(hash, password string) (bool, error) {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("malformed password hash")
	}
	params := map[string]int{}
	for _, kv := range strings.Split(parts[3], ",") {
		key, val, ok := strings.Cut(kv, "=")
		if !ok {
			return false, errors.New("malformed password hash params")
		}
		n, err := strconv.Atoi(val)
		if err != nil {
			return false, errors.New("malformed password hash params")
		}
		params[key] = n
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}
	expected, err := enc.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}
	actual := argon2.IDKey([]byte(password), salt, uint32(params["t"]), uint32(params["m"]), uint8(params["p"]), uint32(len(expected)))
	return subtle.ConstantTimeCompare(expected, actual) == 1, nil
}
