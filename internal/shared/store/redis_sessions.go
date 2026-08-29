package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/redis/go-redis/v9"
)

// sessionKeyPrefix namespaces all session keys so a shared Redis instance
// never collides with other data.
const sessionKeyPrefix = "zelo:session:"

// sessionJSON mirrors model.Session for JSON encoding in Redis.
type sessionJSON struct {
	TokenHash     string    `json:"token_hash"`
	UserID        string    `json:"user_id"`
	CondominiumID string    `json:"condominium_id"`
	CSRFToken     string    `json:"csrf_token"`
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// RedisSessionStore implements security.SessionRepository on top of Redis.
type RedisSessionStore struct {
	client *redis.Client
}

// NewRedisSessionStore creates a RedisSessionStore.
func NewRedisSessionStore(client *redis.Client) *RedisSessionStore {
	return &RedisSessionStore{client: client}
}

// sessionKey returns the Redis key for a session token hash.
func sessionKey(tokenHash string) string {
	return sessionKeyPrefix + tokenHash
}

// CreateSession stores the session as JSON with a TTL matching the remaining
// lifetime, so Redis expires the key automatically.
func (s *RedisSessionStore) CreateSession(ctx context.Context, sess model.Session) error {
	data, err := json.Marshal(toSessionJSON(sess))
	if err != nil {
		return fmt.Errorf("create session: encode: %w", err)
	}
	if err := s.client.Set(ctx, sessionKey(sess.TokenHash), data, ttlFor(sess.ExpiresAt)).Err(); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// GetSessionByTokenHash returns the stored session, or ErrNotFound when the
// key is missing. It defensively discards sessions whose expires_at has passed
// even if Redis has not evicted them yet.
func (s *RedisSessionStore) GetSessionByTokenHash(ctx context.Context, tokenHash string) (model.Session, error) {
	data, err := s.client.Get(ctx, sessionKey(tokenHash)).Bytes()
	if errors.Is(err, redis.Nil) {
		return model.Session{}, ErrNotFound
	}
	if err != nil {
		return model.Session{}, fmt.Errorf("get session: %w", err)
	}
	var sj sessionJSON
	if err := json.Unmarshal(data, &sj); err != nil {
		return model.Session{}, fmt.Errorf("get session: decode: %w", err)
	}
	if !time.Now().Before(sj.ExpiresAt) {
		_ = s.client.Del(ctx, sessionKey(tokenHash))
		return model.Session{}, ErrNotFound
	}
	return toModelSession(sj), nil
}

// UpdateSessionExpiry updates the stored expiry and resets the TTL to the new
// remaining lifetime.
func (s *RedisSessionStore) UpdateSessionExpiry(ctx context.Context, tokenHash string, expiresAt time.Time) error {
	key := sessionKey(tokenHash)
	data, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("update session expiry: %w", err)
	}
	var sj sessionJSON
	if err := json.Unmarshal(data, &sj); err != nil {
		return fmt.Errorf("update session expiry: decode: %w", err)
	}
	sj.ExpiresAt = expiresAt
	updated, err := json.Marshal(sj)
	if err != nil {
		return fmt.Errorf("update session expiry: encode: %w", err)
	}
	if err := s.client.Set(ctx, key, updated, ttlFor(expiresAt)).Err(); err != nil {
		return fmt.Errorf("update session expiry: %w", err)
	}
	return nil
}

// DeleteSession removes the session key. Deleting a missing key is not an
// error so logout stays idempotent.
func (s *RedisSessionStore) DeleteSession(ctx context.Context, tokenHash string) error {
	if err := s.client.Del(ctx, sessionKey(tokenHash)).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// ttlFor returns the TTL for a session expiring at the given time, at least
// one second, so a value is never written without an expiry.
func ttlFor(expiresAt time.Time) time.Duration {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return time.Second
	}
	return time.Duration(math.Ceil(ttl.Seconds())) * time.Second
}

func toSessionJSON(sess model.Session) sessionJSON {
	return sessionJSON{
		TokenHash:     sess.TokenHash,
		UserID:        sess.UserID,
		CondominiumID: sess.CondominiumID,
		CSRFToken:     sess.CSRFToken,
		CreatedAt:     sess.CreatedAt,
		ExpiresAt:     sess.ExpiresAt,
	}
}

func toModelSession(sj sessionJSON) model.Session {
	return model.Session{
		TokenHash:     sj.TokenHash,
		UserID:        sj.UserID,
		CondominiumID: sj.CondominiumID,
		CSRFToken:     sj.CSRFToken,
		CreatedAt:     sj.CreatedAt,
		ExpiresAt:     sj.ExpiresAt,
	}
}
