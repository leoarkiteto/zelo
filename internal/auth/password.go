// Package auth implements password hashing, sessions, and CSRF helpers.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
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

// PasswordHasher hashes and verifies passwords with Argon2id and a site pepper.
//
// The pepper (a secret kept out of the database, e.g. in .env) is applied to
// the password via HMAC-SHA256 before Argon2id, per the OWASP Password Storage
// Cheat Sheet, so a database leak alone does not allow offline guessing.
// Hashes are stored in PHC string format and contain no pepper material.
type PasswordHasher struct {
	pepper []byte
}

// NewPasswordHasher returns a PasswordHasher bound to the given site pepper.
// Pass the same secret from configuration on every boot; changing it makes all
// previously stored password hashes unverifiable. Config validation enforces a
// non-empty pepper at startup.
func NewPasswordHasher(pepper string) PasswordHasher {
	return PasswordHasher{pepper: []byte(pepper)}
}

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

// derive applies the pepper to the password using HMAC-SHA256, with the pepper
// as the key. This binds every hash to the secret without altering the PHC
// string stored in the database.
func (h PasswordHasher) derive(password string) []byte {
	mac := hmac.New(sha256.New, h.pepper)
	mac.Write([]byte(password))
	return mac.Sum(nil)
}

// Hash returns a PHC-formatted Argon2id hash for the password.
func (h PasswordHasher) Hash(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey(h.derive(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// Verify checks a password against a PHC hash using a constant-time compare.
func (h PasswordHasher) Verify(hash, password string) (bool, error) {
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
	actual := argon2.IDKey(h.derive(password), salt, uint32(params["t"]), uint32(params["m"]), uint8(params["p"]), uint32(len(expected)))
	return subtle.ConstantTimeCompare(expected, actual) == 1, nil
}
