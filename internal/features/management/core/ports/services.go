package ports

// TokenHasher hashes opaque tokens before storage.
type TokenHasher interface {
	HashToken(token string) string
}
