package middleware

import (
	"context"
	"net/http"

	"github.com/leoarkiteto/zelo/internal/shared/model"
)

type ctxKey int

const (
	userKey ctxKey = iota
	sessionKey
)

// UserLoader loads users by id.
type UserLoader interface {
	GetUserByID(ctx context.Context, id string) (model.User, error)
}

// RoleChecker returns active roles for a user in a condominium.
type RoleChecker interface {
	ActiveRolesForUser(ctx context.Context, userID, condominiumID string) ([]model.Role, error)
}

// AuditRecorder records security-relevant events.
type AuditRecorder interface {
	RecordEvent(ctx context.Context, userID *string, eventType model.AuditEventType, details map[string]any) error
}

// WithUser authenticates the request when a valid session exists and stores
// the user and session in the request context. Anonymous requests pass through.
func WithUser(sr SessionReader, users UserLoader) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, err := sr.Read(r)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			u, err := users.GetUserByID(r.Context(), sess.UserID)
			if err != nil || u.Status != model.UserStatusActive {
				next.ServeHTTP(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), userKey, u)
			ctx = context.WithValue(ctx, sessionKey, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth redirects unauthenticated browser requests to /login.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFrom(r.Context()) == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole restricts a handler to users holding any of the allowed roles.
func RequireRole(roles RoleChecker, audit AuditRecorder, allowed ...model.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := UserFrom(r.Context())
			sess := SessionFrom(r.Context())
			if u == nil || sess == nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			if sess.CondominiumID == "" {
				deny(w, r, audit, u, sess, allowed)
				return
			}
			roles, err := roles.ActiveRolesForUser(r.Context(), u.ID, sess.CondominiumID)
			if err != nil {
				deny(w, r, audit, u, sess, allowed)
				return
			}
			for _, have := range roles {
				for _, want := range allowed {
					if have == want {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
			deny(w, r, audit, u, sess, allowed)
		})
	}
}

func deny(w http.ResponseWriter, r *http.Request, audit AuditRecorder, u *model.User, sess *model.Session, allowed []model.Role) {
	if audit != nil {
		details := map[string]any{"path": r.URL.Path, "allowed_roles": allowed}
		uid := u.ID
		_ = audit.RecordEvent(r.Context(), &uid, model.AuditAccessDenied, details)
	}
	http.Error(w, "Forbidden: you do not have access to this area", http.StatusForbidden)
}

// UserFrom returns the authenticated user from the context, or nil.
func UserFrom(ctx context.Context) *model.User {
	u, _ := ctx.Value(userKey).(model.User)
	if u.ID == "" {
		return nil
	}
	return &u
}

// SessionFrom returns the session from the context, or nil.
func SessionFrom(ctx context.Context) *model.Session {
	s, _ := ctx.Value(sessionKey).(model.Session)
	if s.TokenHash == "" {
		return nil
	}
	return &s
}
