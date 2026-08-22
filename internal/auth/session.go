package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/leoarkiteto/zelo/internal/model"
)

// SessionCookieName is the name of the session cookie.
const SessionCookieName = "zelo_session"

var (
	// ErrNoSession is returned when no valid cookie is present.
	ErrNoSession = errors.New("no session")
	// ErrSessionExpired is returned when the session has expired.
	ErrSessionExpired = errors.New("session expired")
)

// SessionRepository persists session rows.
type SessionRepository interface {
	CreateSession(ctx context.Context, sess model.Session) error
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (model.Session, error)
	UpdateSessionExpiry(ctx context.Context, tokenHash string, expiresAt time.Time) error
	DeleteSession(ctx context.Context, tokenHash string) error
}

// SessionManager issues and validates server-side sessions.
type SessionManager struct {
	Repo        SessionRepository
	Secure      bool
	Lifetime    time.Duration // absolute lifetime (7 days)
	IdleTimeout time.Duration // idle timeout (12 hours)
	Now         func() time.Time
}

// NewSessionManager creates a SessionManager with default timeouts.
func NewSessionManager(repo SessionRepository, secure bool) *SessionManager {
	return &SessionManager{
		Repo:        repo,
		Secure:      secure,
		Lifetime:    7 * 24 * time.Hour,
		IdleTimeout: 12 * time.Hour,
		Now:         time.Now,
	}
}

// Create issues a new session for the user and returns the raw cookie value
// plus the CSRF token to render in forms.
func (m *SessionManager) Create(ctx context.Context, userID, condominiumID string) (rawID, csrfToken string, err error) {
	rawID, err = randomToken()
	if err != nil {
		return "", "", err
	}
	csrfToken, err = randomToken()
	if err != nil {
		return "", "", err
	}
	sess := model.Session{
		TokenHash:     HashToken(rawID),
		UserID:        userID,
		CondominiumID: condominiumID,
		CSRFToken:     csrfToken,
		ExpiresAt:     m.Now().Add(m.Lifetime),
	}
	if err := m.Repo.CreateSession(ctx, sess); err != nil {
		return "", "", fmt.Errorf("create session: %w", err)
	}
	return rawID, csrfToken, nil
}

// Cookie builds the session cookie for the raw session ID.
func (m *SessionManager) Cookie(rawID string) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    rawID,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  m.Now().Add(m.Lifetime),
	}
}

// Read validates the session from the request cookie and refreshes the idle timeout.
func (m *SessionManager) Read(r *http.Request) (model.Session, error) {
	c, err := r.Cookie(SessionCookieName)
	if err != nil {
		return model.Session{}, ErrNoSession
	}
	sess, err := m.Repo.GetSessionByTokenHash(r.Context(), HashToken(c.Value))
	if err != nil {
		return model.Session{}, ErrNoSession
	}
	now := m.Now()
	if !sess.ExpiresAt.After(now) {
		return model.Session{}, ErrSessionExpired
	}
	if idle := now.Add(m.IdleTimeout); idle.Before(sess.ExpiresAt) {
		_ = m.Repo.UpdateSessionExpiry(r.Context(), sess.TokenHash, idle)
	}
	return sess, nil
}

// Invalidate deletes the session row and clears the cookie.
func (m *SessionManager) Invalidate(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookieName); err == nil {
		_ = m.Repo.DeleteSession(r.Context(), HashToken(c.Value))
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

// randomToken returns a base64url-encoded 32-byte random token.
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken returns the hex SHA-256 of a token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
