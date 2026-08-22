// Package security provides password hashing, sessions, CSRF, and token primitives.
package security

// TokenHasher adapts HashToken to service TokenHasher ports.
type TokenHasher struct{}

// HashToken hashes an opaque token with SHA-256.
func (TokenHasher) HashToken(token string) string { return HashToken(token) }
