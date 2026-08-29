package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/redis/go-redis/v9"
)

func newRedisSessionTestStore(t *testing.T) (*RedisSessionStore, *miniredis.Miniredis) {
	t.Helper()
	m := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: m.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisSessionStore(client), m
}

func TestRedisSessionStoreCreateAndGet(t *testing.T) {
	ctx := context.Background()
	s, _ := newRedisSessionTestStore(t)
	now := time.Now().UTC()
	sess := model.Session{
		TokenHash:     "abc123",
		UserID:        "user-1",
		CondominiumID: "condo-1",
		CSRFToken:     "csrf-token-xyz",
		CreatedAt:     now,
		ExpiresAt:     now.Add(1 * time.Hour),
	}

	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	got, err := s.GetSessionByTokenHash(ctx, sess.TokenHash)
	if err != nil {
		t.Fatalf("GetSessionByTokenHash() error = %v", err)
	}
	if got != sess {
		t.Fatalf("GetSessionByTokenHash() = %+v, want %+v", got, sess)
	}
	if got.CSRFToken != "csrf-token-xyz" {
		t.Fatalf("CSRFToken = %q, want csrf-token-xyz", got.CSRFToken)
	}
}

func TestRedisSessionStoreGetMissingKeyReturnsNotFound(t *testing.T) {
	s, _ := newRedisSessionTestStore(t)
	if _, err := s.GetSessionByTokenHash(context.Background(), "does-not-exist"); err != ErrNotFound {
		t.Fatalf("GetSessionByTokenHash() error = %v, want ErrNotFound", err)
	}
}

func TestRedisSessionStoreCreateSetsTTL(t *testing.T) {
	ctx := context.Background()
	s, m := newRedisSessionTestStore(t)
	sess := model.Session{
		TokenHash: "ttl-key",
		UserID:    "user-1",
		CSRFToken: "csrf",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: m.Addr()})
	defer client.Close()
	ttl, err := client.TTL(ctx, sessionKey(sess.TokenHash)).Result()
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	if ttl <= 0 || ttl > time.Hour {
		t.Fatalf("TTL = %v, want within (0, 1h]", ttl)
	}
}

func TestRedisSessionStoreUpdateExpiry(t *testing.T) {
	ctx := context.Background()
	s, m := newRedisSessionTestStore(t)
	now := time.Now().UTC()
	sess := model.Session{
		TokenHash: "refresh-key",
		UserID:    "user-1",
		CSRFToken: "csrf",
		CreatedAt: now,
		ExpiresAt: now.Add(1 * time.Hour),
	}
	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	newExpiry := now.Add(2 * time.Hour)
	if err := s.UpdateSessionExpiry(ctx, sess.TokenHash, newExpiry); err != nil {
		t.Fatalf("UpdateSessionExpiry() error = %v", err)
	}

	got, err := s.GetSessionByTokenHash(ctx, sess.TokenHash)
	if err != nil {
		t.Fatalf("GetSessionByTokenHash() error = %v", err)
	}
	if !got.ExpiresAt.Equal(newExpiry) {
		t.Fatalf("ExpiresAt = %v, want %v", got.ExpiresAt, newExpiry)
	}
	client := redis.NewClient(&redis.Options{Addr: m.Addr()})
	defer client.Close()
	ttl, err := client.TTL(ctx, sessionKey(sess.TokenHash)).Result()
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	if ttl <= time.Hour {
		t.Fatalf("TTL = %v, want extended beyond 1h", ttl)
	}
}

func TestRedisSessionStoreConcurrentReadsAndRefreshes(t *testing.T) {
	ctx := context.Background()
	s, _ := newRedisSessionTestStore(t)
	now := time.Now().UTC()
	sess := model.Session{
		TokenHash: "concurrent-key",
		UserID:    "user-1",
		CSRFToken: "csrf",
		CreatedAt: now,
		ExpiresAt: now.Add(1 * time.Hour),
	}
	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := s.GetSessionByTokenHash(ctx, sess.TokenHash); err != nil {
				errs <- err
			}
		}()
		go func() {
			defer wg.Done()
			if err := s.UpdateSessionExpiry(ctx, sess.TokenHash, time.Now().Add(2*time.Hour)); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent access error = %v", err)
	}

	got, err := s.GetSessionByTokenHash(ctx, sess.TokenHash)
	if err != nil {
		t.Fatalf("final GetSessionByTokenHash() error = %v", err)
	}
	if got.UserID != "user-1" {
		t.Fatalf("final UserID = %q, want user-1", got.UserID)
	}
}

func TestRedisSessionStoreMultipleSessionsIndependent(t *testing.T) {
	ctx := context.Background()
	s, _ := newRedisSessionTestStore(t)
	now := time.Now().UTC()
	sessA := model.Session{
		TokenHash: "session-a",
		UserID:    "user-1",
		CSRFToken: "csrf-a",
		CreatedAt: now,
		ExpiresAt: now.Add(1 * time.Hour),
	}
	sessB := model.Session{
		TokenHash: "session-b",
		UserID:    "user-1",
		CSRFToken: "csrf-b",
		CreatedAt: now,
		ExpiresAt: now.Add(1 * time.Hour),
	}
	if err := s.CreateSession(ctx, sessA); err != nil {
		t.Fatalf("create A: %v", err)
	}
	if err := s.CreateSession(ctx, sessB); err != nil {
		t.Fatalf("create B: %v", err)
	}
	if err := s.DeleteSession(ctx, sessA.TokenHash); err != nil {
		t.Fatalf("delete A: %v", err)
	}
	if _, err := s.GetSessionByTokenHash(ctx, sessA.TokenHash); err != ErrNotFound {
		t.Fatalf("Get A error = %v, want ErrNotFound", err)
	}
	gotB, err := s.GetSessionByTokenHash(ctx, sessB.TokenHash)
	if err != nil {
		t.Fatalf("Get B error = %v", err)
	}
	if gotB.CSRFToken != "csrf-b" {
		t.Fatalf("B CSRFToken = %q, want csrf-b", gotB.CSRFToken)
	}
}

func TestRedisSessionStoreDeleteSessionRemovesKey(t *testing.T) {
	ctx := context.Background()
	s, m := newRedisSessionTestStore(t)
	sess := model.Session{
		TokenHash: "delete-key",
		UserID:    "user-1",
		CSRFToken: "csrf",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := s.DeleteSession(ctx, sess.TokenHash); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}
	if _, err := s.GetSessionByTokenHash(ctx, sess.TokenHash); err != ErrNotFound {
		t.Fatalf("Get after delete error = %v, want ErrNotFound", err)
	}
	client := redis.NewClient(&redis.Options{Addr: m.Addr()})
	defer client.Close()
	if n, _ := client.Exists(ctx, sessionKey(sess.TokenHash)).Result(); n != 0 {
		t.Fatalf("key still exists after delete (exists = %d)", n)
	}
}

func TestRedisSessionStoreDeleteSessionIdempotent(t *testing.T) {
	ctx := context.Background()
	s, _ := newRedisSessionTestStore(t)
	sess := model.Session{
		TokenHash: "missing-key",
		UserID:    "user-1",
		CSRFToken: "csrf",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	if err := s.DeleteSession(ctx, sess.TokenHash); err != nil {
		t.Fatalf("DeleteSession() on missing key error = %v, want nil", err)
	}
}

func TestRedisSessionStoreTTLExpiresKeyAutomatically(t *testing.T) {
	ctx := context.Background()
	s, m := newRedisSessionTestStore(t)
	sess := model.Session{
		TokenHash: "ttl-expiry-key",
		UserID:    "user-1",
		CSRFToken: "csrf",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Fast-forward past the TTL; Redis evicts the key automatically.
	m.FastForward(61 * time.Minute)

	if _, err := s.GetSessionByTokenHash(ctx, sess.TokenHash); err != ErrNotFound {
		t.Fatalf("Get after expiry error = %v, want ErrNotFound", err)
	}
	client := redis.NewClient(&redis.Options{Addr: m.Addr()})
	defer client.Close()
	if n, _ := client.Exists(ctx, sessionKey(sess.TokenHash)).Result(); n != 0 {
		t.Fatalf("key still exists after expiry (exists = %d)", n)
	}
}
