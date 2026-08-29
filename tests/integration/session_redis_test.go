package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/shared/security"
	"github.com/leoarkiteto/zelo/internal/shared/store"
	"github.com/leoarkiteto/zelo/internal/shared/testutil"
	"github.com/redis/go-redis/v9"
)

// redisTestClient opens a client on the same Redis DB used by newApp, skipping
// when TEST_REDIS_URL is unset.
func redisTestClient(t *testing.T) *redis.Client {
	t.Helper()
	u := testutil.TestRedisURL("integration")
	if u == "" {
		t.Skip("TEST_REDIS_URL not set; skipping integration test")
	}
	c, err := store.OpenRedis(u)
	if err != nil {
		t.Fatalf("open redis: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func sessionKeys(t *testing.T, c *redis.Client) []string {
	t.Helper()
	iter := c.Scan(context.Background(), 0, "zelo:session:*", 100).Iterator()
	var keys []string
	for iter.Next(context.Background()) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		t.Fatalf("scan session keys: %v", err)
	}
	return keys
}

func TestIntegrationSessionStoredInRedisAndIdleRefresh(t *testing.T) {
	router := newApp(t)
	ts := httptest.NewServer(router)
	defer ts.Close()
	base := ts.URL

	c := redisTestClient(t)
	ctx := context.Background()

	// Login as the syndic.
	client := newClient(t)
	code, _ := postForm(t, client, base, "/login", map[string]string{
		"email": "syndic@example.com", "password": "syndic-pass-123",
		"csrf_token": csrfFrom(t, client, base, "/login"),
	})
	if code != http.StatusSeeOther {
		t.Fatalf("login = %d, want 303", code)
	}

	keys := sessionKeys(t, c)
	if len(keys) != 1 {
		t.Fatalf("redis session keys = %v, want exactly 1", keys)
	}
	key := keys[0]

	ttlAfterLogin, err := c.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("TTL after login: %v", err)
	}
	if ttlAfterLogin <= 6*24*time.Hour || ttlAfterLogin > 7*24*time.Hour {
		t.Fatalf("TTL after login = %v, want ~7-day absolute lifetime", ttlAfterLogin)
	}

	// A protected request works and applies the 12-hour idle window.
	code, _ = getPage(t, client, base, "/invitations")
	if code != http.StatusOK {
		t.Fatalf("protected /invitations = %d, want 200", code)
	}
	ttlAfterIdle, err := c.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("TTL after idle refresh: %v", err)
	}
	if ttlAfterIdle <= 11*time.Hour || ttlAfterIdle > 12*time.Hour {
		t.Fatalf("TTL after idle refresh = %v, want ~12-hour idle window", ttlAfterIdle)
	}
	if keys := sessionKeys(t, c); len(keys) != 1 {
		t.Fatalf("session keys after refresh = %v, want still exactly 1", keys)
	}

	// Waiting and requesting again keeps the session valid; the TTL stays
	// within the 12-hour idle window (set on first authenticated use).
	time.Sleep(2 * time.Second)
	code, _ = getPage(t, client, base, "/invitations")
	if code != http.StatusOK {
		t.Fatalf("protected /invitations after wait = %d, want 200", code)
	}
	ttlAfterWait, err := c.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("TTL after wait: %v", err)
	}
	if ttlAfterWait <= 11*time.Hour || ttlAfterWait > 12*time.Hour {
		t.Fatalf("TTL after wait = %v, want within the 12-hour idle window", ttlAfterWait)
	}

	// The login page still renders the authenticated dashboard for the syndic.
	code, body := getPage(t, client, base, "/")
	if code != http.StatusOK || !strings.Contains(body, "syndic") {
		t.Fatalf("dashboard = %d, want 200 containing syndic", code)
	}
}

func TestIntegrationLogoutDeletesRedisSession(t *testing.T) {
	router := newApp(t)
	ts := httptest.NewServer(router)
	defer ts.Close()
	base := ts.URL
	c := redisTestClient(t)

	client := newClient(t)
	code, _ := postForm(t, client, base, "/login", map[string]string{
		"email": "syndic@example.com", "password": "syndic-pass-123",
		"csrf_token": csrfFrom(t, client, base, "/login"),
	})
	if code != http.StatusSeeOther {
		t.Fatalf("login = %d, want 303", code)
	}
	keys := sessionKeys(t, c)
	if len(keys) != 1 {
		t.Fatalf("session keys = %v, want 1", keys)
	}

	// Logout with the CSRF token from an authenticated page.
	code, body := getPage(t, client, base, "/invitations")
	if code != http.StatusOK {
		t.Fatalf("invitations = %d, want 200", code)
	}
	csrf := csrfRe.FindStringSubmatch(body)
	if csrf == nil {
		t.Fatal("no csrf token on invitations page")
	}
	code, _ = postForm(t, client, base, "/logout", map[string]string{"csrf_token": csrf[1]})
	if code != http.StatusSeeOther {
		t.Fatalf("logout = %d, want 303", code)
	}

	// The session key is gone from Redis and the old cookie is rejected.
	if keys := sessionKeys(t, c); len(keys) != 0 {
		t.Fatalf("session keys after logout = %v, want 0", keys)
	}
	code, _ = getPage(t, client, base, "/invitations")
	if code != http.StatusSeeOther {
		t.Fatalf("protected page after logout = %d, want 303 redirect to login", code)
	}

	// Logout without a valid session completes without error.
	anon := newClient(t)
	code, _ = postForm(t, anon, base, "/logout", map[string]string{
		"csrf_token": csrfFrom(t, anon, base, "/login"),
	})
	if code != http.StatusSeeOther {
		t.Fatalf("anonymous logout = %d, want 303", code)
	}
}

func TestIntegrationSessionSurvivesAppRestartNotRedisRestart(t *testing.T) {
	c := redisTestClient(t)
	ctx := context.Background()
	if err := c.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush redis: %v", err)
	}
	t.Cleanup(func() { _ = c.FlushDB(ctx).Err() })

	repo := store.NewRedisSessionStore(c)
	mgr1 := security.NewSessionManager(repo, false)
	rawID, _, err := mgr1.Create(ctx, "user-1", "condo-1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	cookie := mgr1.Cookie(rawID)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	if _, err := mgr1.Read(req); err != nil {
		t.Fatalf("Read() with original manager error = %v", err)
	}

	// Application restart: a fresh SessionManager over the same Redis store
	// still validates the cookie.
	mgr2 := security.NewSessionManager(repo, false)
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(cookie)
	sess, err := mgr2.Read(req2)
	if err != nil {
		t.Fatalf("Read() after app restart error = %v, want session still valid", err)
	}
	if sess.UserID != "user-1" {
		t.Fatalf("UserID after app restart = %q, want user-1", sess.UserID)
	}

	// Redis restart/loss: flushing the data store invalidates the session.
	if err := c.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush redis: %v", err)
	}
	if _, err := mgr2.Read(req2); err == nil {
		t.Fatal("Read() after Redis flush = nil error, want session lost")
	}
}
