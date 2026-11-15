package handlers

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/features/invoices/services"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/templates/organisms"
)

// actor returns the signed-in user id and the session's condominium id. The
// role guard guarantees both are set.
func actor(r *http.Request) (userID, condoID string) {
	return httpx.CurrentUser(r).ID, httpx.CurrentSession(r).CondominiumID
}

func locale(r *http.Request) i18n.Language { return i18n.LanguageFrom(r.Context()) }

func tr(r *http.Request, key string) string { return i18n.T(locale(r), i18n.MessageKey(key)) }

// shell builds the app shell from the roles the guard stored in the context.
func shell(r *http.Request, active string) (*organisms.ShellData, error) {
	return httpx.ShellDataWithRoles(r, active, middleware.RolesFrom(r.Context()))
}

// fail renders the shared error page with a translated message.
func fail(w http.ResponseWriter, r *http.Request, status int, key string) {
	httpx.RenderError(w, r, status, tr(r, key))
}

func notFound(w http.ResponseWriter, r *http.Request) {
	fail(w, r, http.StatusNotFound, "invoices.error.not_found")
}

func serverError(w http.ResponseWriter, r *http.Request, msg string, err error, key string) {
	slog.Error("invoices: "+msg, "error", err)
	fail(w, r, http.StatusInternalServerError, key)
}

// isNotFound reports a record that is missing or belongs to another condominium.
func isNotFound(err error) bool { return errors.Is(err, services.ErrNotFound) }

// flashText maps a ?flash= value to its message, accepting only the listed
// names. prefix is the catalog namespace, e.g. "invoices.flash.".
func flashText(r *http.Request, prefix string, allowed ...string) string {
	name := r.URL.Query().Get("flash")
	if name == "" || !slices.Contains(allowed, name) {
		return ""
	}
	return tr(r, prefix+name)
}

// newIdempotencyKey returns a random UUID (v4) for the create form.
func newIdempotencyKey() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ruleKey returns the i18n key of a domain rule error, or "".
func ruleKey(err error) string { return domain.ErrorKey(err) }
