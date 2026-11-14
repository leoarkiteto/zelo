package handlers

import (
	"log"
	"net/http"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	financetemplates "github.com/leoarkiteto/zelo/internal/features/finance/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

var memberRoles = []model.Role{model.RoleOwner, model.RoleTenant, model.RoleSyndic}

func isHX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

// hxAwareMemberOnly keeps the standard RequireAuth/RequireRole chain for plain
// requests. htmx v4 (web/static/js/htmx.min.js) swaps every response status
// except 204/304 into the target, so for HX requests the standard 303 redirect
// and plain-text 403 would be injected (or followed) inside #finance-overview.
// This wrapper replaces those responses with htmx-friendly ones.
func (h *Handler) hxAwareMemberOnly(next http.Handler) http.Handler {
	standard := middleware.RequireAuth(
		middleware.RequireRole(h.deps.Roles, h.deps.Audit, memberRoles...)(next))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "HX-Request")
		if !isHX(r) {
			standard.ServeHTTP(w, r)
			return
		}
		u := middleware.UserFrom(r.Context())
		sess := middleware.SessionFrom(r.Context())
		if u == nil || sess == nil {
			// Non-3xx on purpose: fetch would follow a 3xx before htmx could read
			// HX-Redirect. htmx reads t.hx.redirect (the HX-Redirect header)
			// before swapping and navigates, whatever the status. 401 is the
			// accurate status for an unauthenticated request.
			w.Header().Set("HX-Redirect", "/login")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if sess.CondominiumID != "" {
			roles, err := h.deps.Roles.ActiveRolesForUser(r.Context(), u.ID, sess.CondominiumID)
			if err == nil && hasAnyRole(roles, memberRoles) {
				next.ServeHTTP(w, r)
				return
			}
		}
		// Same audit event as middleware.RequireRole's deny.
		if h.deps.Audit != nil {
			uid := u.ID
			_ = h.deps.Audit.RecordEvent(r.Context(), &uid, model.AuditAccessDenied,
				map[string]any{"path": r.URL.Path, "allowed_roles": memberRoles})
		}
		w.WriteHeader(http.StatusForbidden)
		httpx.Render(w, r, financetemplates.OverviewNoAccess(i18n.LanguageFrom(r.Context())))
	})
}

func hasAnyRole(have, want []model.Role) bool {
	for _, a := range have {
		for _, b := range want {
			if a == b {
				return true
			}
		}
	}
	return false
}

// overviewGET renders the financial overview panel (fragment for htmx, full
// page otherwise).
func (h *Handler) overviewGET(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Vary", "HX-Request")
	sess := httpx.CurrentSession(r)
	u := httpx.CurrentUser(r)
	locale := i18n.LanguageFrom(r.Context())
	hx := isHX(r)
	period := domain.ParsePeriod(r.URL.Query().Get("period"), h.now())

	fail := func(err error) {
		log.Printf("finance overview: %v", err)
		if hx {
			// The vendored htmx v4 swaps every status except 204/304, so the 500
			// partial replaces the panel and offers a retry.
			w.WriteHeader(http.StatusInternalServerError)
			httpx.Render(w, r, financetemplates.OverviewError(locale, period.Param()))
			return
		}
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.load"))
	}

	ov, err := h.deps.Finance.Overview(r.Context(), sess.CondominiumID, period)
	if err != nil {
		fail(err)
		return
	}
	roles, err := h.deps.Roles.ActiveRolesForUser(r.Context(), u.ID, sess.CondominiumID)
	if err != nil {
		fail(err)
		return
	}
	data := financetemplates.OverviewData{
		Locale:   locale,
		Overview: ov,
		IsSyndic: hasAnyRole(roles, []model.Role{model.RoleSyndic}),
	}
	if hx {
		httpx.Render(w, r, financetemplates.OverviewPanel(data))
		return
	}
	shell, err := httpx.ShellData(r, h.deps.Roles, "/")
	if err != nil {
		fail(err)
		return
	}
	httpx.Render(w, r, financetemplates.OverviewPage(shell, data))
}

// healthRedirect keeps the retired /finance/health address working (FR-023).
func (h *Handler) healthRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
