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
	rolesKey
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

// RoleGuardResponders customises the guard's failure responses. A nil field
// selects the default behaviour.
type RoleGuardResponders struct {
	// Unauthenticated handles requests without a user/session (nil: 303 to /login).
	Unauthenticated func(w http.ResponseWriter, r *http.Request)
	// Denied handles forbidden requests (nil: plain-text 403). The access_denied
	// audit event is always recorded before it runs.
	Denied func(w http.ResponseWriter, r *http.Request)
}

// RequireRole restricts a handler to users holding any of the allowed roles.
func RequireRole(roles RoleChecker, audit AuditRecorder, allowed ...model.Role) func(http.Handler) http.Handler {
	return RequireRoleWith(roles, audit, RoleGuardResponders{}, allowed...)
}

// RequireRoleWith is RequireRole with injectable unauthenticated/denied
// responses. On allow, the user's active roles are stored in the request
// context (see RolesFrom).
func RequireRoleWith(roles RoleChecker, audit AuditRecorder, resp RoleGuardResponders, allowed ...model.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := UserFrom(r.Context())
			sess := SessionFrom(r.Context())
			if u == nil || sess == nil {
				if resp.Unauthenticated != nil {
					resp.Unauthenticated(w, r)
					return
				}
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			if sess.CondominiumID == "" {
				deny(w, r, audit, resp, u, allowed)
				return
			}
			have, err := roles.ActiveRolesForUser(r.Context(), u.ID, sess.CondominiumID)
			if err != nil {
				deny(w, r, audit, resp, u, allowed)
				return
			}
			for _, h := range have {
				for _, want := range allowed {
					if h == want {
						ctx := context.WithValue(r.Context(), rolesKey, have)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}
			}
			deny(w, r, audit, resp, u, allowed)
		})
	}
}

func deny(w http.ResponseWriter, r *http.Request, audit AuditRecorder, resp RoleGuardResponders, u *model.User, allowed []model.Role) {
	if audit != nil {
		details := map[string]any{"path": r.URL.Path, "allowed_roles": allowed}
		uid := u.ID
		_ = audit.RecordEvent(r.Context(), &uid, model.AuditAccessDenied, details)
	}
	if resp.Denied != nil {
		resp.Denied(w, r)
		return
	}
	http.Error(w, "Forbidden: you do not have access to this area", http.StatusForbidden)
}

// RolesFrom returns the active roles stored by the role guard on allow, or nil.
func RolesFrom(ctx context.Context) []model.Role {
	r, _ := ctx.Value(rolesKey).([]model.Role)
	return r
}

// IsHX reports whether the request was issued by htmx.
func IsHX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// HXRedirect asks htmx to navigate to location. It answers 401, deliberately
// not a 3xx: fetch would follow a 3xx redirect before htmx could read the
// HX-Redirect header. The vendored htmx v4 reads HX-Redirect before swapping,
// at any status.
func HXRedirect(w http.ResponseWriter, location string) {
	w.Header().Set("HX-Redirect", location)
	w.WriteHeader(http.StatusUnauthorized)
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
