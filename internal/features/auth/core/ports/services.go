package ports

// PasswordHasher hashes and verifies passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(hash, password string) (bool, error)
	ValidatePassword(password string) error
}

// TokenHasher hashes opaque tokens before storage.
type TokenHasher interface {
	HashToken(token string) string
}
