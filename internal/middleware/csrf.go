package middleware

import (
	"net/http"

	"github.com/leoarkiteto/zelo/internal/auth"
	"github.com/leoarkiteto/zelo/internal/model"
)

// SessionReader reads and validates the session from a request.
type SessionReader interface {
	Read(r *http.Request) (model.Session, error)
}

// CSRF rejects state-changing requests without a valid token. For
// authenticated sessions the per-session token is checked; for anonymous
// requests a double-submit cookie (`zelo_csrf`) must match the form value.
func CSRF(sr SessionReader) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
				if err := r.ParseForm(); err != nil {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
				submitted := r.FormValue("csrf_token")
				sess, err := sr.Read(r)
				var valid bool
				if err == nil {
					valid = auth.ValidateCSRF(submitted, sess.CSRFToken)
				} else if c, cerr := r.Cookie(anonCSRFCookie); cerr == nil {
					valid = auth.ValidateCSRF(submitted, c.Value)
				}
				if !valid {
					http.Error(w, "Invalid CSRF token", http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// anonCSRFCookie is the double-submit CSRF cookie used on public pages.
const anonCSRFCookie = "zelo_csrf"
