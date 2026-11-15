package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/security"
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
				// r.FormValue silently drops parse errors, so parse explicitly
				// to tell an exceeded body limit (413) from a bad form (403).
				var perr error
				if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
					perr = r.ParseMultipartForm(32 << 20)
				} else {
					perr = r.ParseForm()
				}
				if perr != nil {
					var maxErr *http.MaxBytesError
					if errors.As(perr, &maxErr) {
						http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
						return
					}
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
				submitted := r.FormValue("csrf_token")
				sess, err := sr.Read(r)
				var valid bool
				if err == nil {
					valid = security.ValidateCSRF(submitted, sess.CSRFToken)
				} else if c, cerr := r.Cookie(anonCSRFCookie); cerr == nil {
					valid = security.ValidateCSRF(submitted, c.Value)
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
