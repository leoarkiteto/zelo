package handlers

import (
	"net/http"

	profiletemplates "github.com/leoarkiteto/zelo/internal/features/profile/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	organisms "github.com/leoarkiteto/zelo/internal/shared/templates/organisms"
)

// profileGET renders the profile page containing the language toggle.
func (h *Handler) profileGET(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	shell, err := httpx.ShellData(r, h.deps.Roles, "/profile")
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(i18n.LanguageFrom(r.Context()), "profile.error.load_failed"))
		return
	}
	prof, err := h.deps.Profile.GetProfile(r.Context(), u.ID)
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(i18n.LanguageFrom(r.Context()), "profile.error.load_failed"))
		return
	}
	httpx.Render(w, r, profiletemplates.ProfilePage(profiletemplates.ProfilePageData{
		Shell:           shell,
		CurrentLanguage: prof.Language,
	}))
}

// profileLanguagePOST persists the chosen language. HTMX requests receive the
// re-rendered authenticated shell in the new locale; plain forms redirect
// back to /profile (contracts/http-endpoints.md).
func (h *Handler) profileLanguagePOST(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	lang := i18n.Language(r.FormValue("language"))
	locale := i18n.LanguageFrom(r.Context())
	if !lang.Valid() {
		httpx.RenderError(w, r, http.StatusBadRequest, i18n.T(locale, "profile.error.invalid_language"))
		return
	}
	if err := h.deps.Profile.ChangeLanguage(r.Context(), u.ID, lang); err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "profile.error.save_failed"))
		return
	}

	if r.Header.Get("HX-Request") != "true" {
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// Re-render the whole authenticated shell in the new locale so HTMX can
	// swap it in place (one round-trip, no page state lost).
	shell, err := httpx.ShellData(r, h.deps.Roles, "/profile")
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "profile.error.save_failed"))
		return
	}
	shell.Locale = lang
	if sess := httpx.CurrentSession(r); sess != nil {
		if roles, err := h.deps.Roles.ActiveRolesForUser(r.Context(), u.ID, sess.CondominiumID); err == nil {
			shell.Nav = httpx.NavFor(roles, "/profile", lang)
		}
	}
	httpx.Render(w, r, organisms.Shell(shell, profiletemplates.ProfileBody(profiletemplates.ProfilePageData{
		Shell:           shell,
		CurrentLanguage: lang,
	})))
}
