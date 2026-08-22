package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/model"
)

type fakeSessionRepo struct {
	sessions map[string]model.Session
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{sessions: map[string]model.Session{}}
}

func (f *fakeSessionRepo) CreateSession(_ context.Context, s model.Session) error {
	f.sessions[s.TokenHash] = s
	return nil
}

func (f *fakeSessionRepo) GetSessionByTokenHash(_ context.Context, h string) (model.Session, error) {
	s, ok := f.sessions[h]
	if !ok {
		return model.Session{}, errNotFound
	}
	return s, nil
}

func (f *fakeSessionRepo) UpdateSessionExpiry(_ context.Context, h string, exp time.Time) error {
	s, ok := f.sessions[h]
	if !ok {
		return errNotFound
	}
	s.ExpiresAt = exp
	f.sessions[h] = s
	return nil
}

func (f *fakeSessionRepo) DeleteSession(_ context.Context, h string) error {
	delete(f.sessions, h)
	return nil
}

var errNotFound = &notFoundError{}

type notFoundError struct{}

func (*notFoundError) Error() string { return "not found" }

func TestSessionManagerCreateReadInvalidate(t *testing.T) {
	repo := newFakeSessionRepo()
	m := NewSessionManager(repo, false)
	now := time.Now()
	m.Now = func() time.Time { return now }

	rawID, csrf, err := m.Create(context.Background(), "user-1", "condo-1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if rawID == "" || csrf == "" {
		t.Fatal("Create() returned empty token")
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(m.Cookie(rawID))
	sess, err := m.Read(req)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if sess.UserID != "user-1" {
		t.Fatalf("Read() userID = %q, want user-1", sess.UserID)
	}

	// Unknown cookie -> no session.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "unknown"})
	if _, err := m.Read(req2); err == nil {
		t.Fatal("Read() = nil error for unknown session")
	}

	// Expired session -> ErrSessionExpired.
	expired := m.Now().Add(-1 * time.Hour)
	if err := repo.UpdateSessionExpiry(context.Background(), HashToken(rawID), expired); err != nil {
		t.Fatalf("expire session: %v", err)
	}
	if _, err := m.Read(req); err == nil {
		t.Fatal("Read() = nil error for expired session")
	}

	// Invalidate removes the row and clears the cookie.
	m.Now = func() time.Time { return time.Now() }
	rec := httptest.NewRecorder()
	m.Invalidate(rec, req)
	if _, ok := repo.sessions[HashToken(rawID)]; ok {
		t.Fatal("Invalidate() left session row behind")
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || cookies[0].MaxAge != -1 {
		t.Fatal("Invalidate() did not clear the cookie")
	}
}
