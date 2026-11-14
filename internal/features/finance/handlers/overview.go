package handlers

import (
	"log"
	"net/http"

	financetemplates "github.com/leoarkiteto/zelo/internal/features/finance/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

var memberRoles = []model.Role{model.RoleOwner, model.RoleTenant, model.RoleSyndic}

// hxAwareMemberOnly guards the overview. htmx v4 swaps every response status
// except 204/304, so for HX requests the standard 303 redirect and plain-text
// 403 would be injected (or followed) inside #finance-overview; HX requests get
// htmx-friendly responses instead. Plain requests keep the standard chain.
func (h *Handler) hxAwareMemberOnly(next http.Handler) http.Handler {
	standard := middleware.RequireAuth(
		middleware.RequireRole(h.deps.Roles, h.deps.Audit, memberRoles...)(next))
	hxGuard := middleware.RequireRoleWith(h.deps.Roles, h.deps.Audit, middleware.RoleGuardResponders{
		Unauthenticated: func(w http.ResponseWriter, r *http.Request) {
			middleware.HXRedirect(w, "/login")
		},
		Denied: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			httpx.Render(w, r, financetemplates.OverviewNoAccess(i18n.LanguageFrom(r.Context())))
		},
	}, memberRoles...)(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "HX-Request")
		if middleware.IsHX(r) {
			hxGuard.ServeHTTP(w, r)
			return
		}
		standard.ServeHTTP(w, r)
	})
}

// overviewGET renders the financial overview panel (fragment for htmx, full
// page otherwise).
func (h *Handler) overviewGET(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Vary", "HX-Request")
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())
	hx := middleware.IsHX(r)
	period := h.deps.Finance.ResolvePeriod(r.URL.Query().Get("period"))

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
	roles := middleware.RolesFrom(r.Context())
	isSyndic := false
	for _, role := range roles {
		if role == model.RoleSyndic {
			isSyndic = true
			break
		}
	}
	data := financetemplates.OverviewData{
		Locale:   locale,
		Overview: ov,
		IsSyndic: isSyndic,
	}
	if hx {
		httpx.Render(w, r, financetemplates.OverviewPanel(data))
		return
	}
	shell, err := httpx.ShellDataWithRoles(r, "/", roles)
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
