package handlers

import (
	"net/http"

	"github.com/leoarkiteto/zelo/internal/features/home/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

func (h *Handler) condominium(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	h.renderArea(w, r, "/condominium", templates.AreaPageData{
		Title:   i18n.T(locale, "area.condominium_title"),
		Message: i18n.T(locale, "area.condominium_message"),
		Links: []templates.DashboardLink{
			{Label: i18n.T(locale, "nav.invitations"), Path: "/invitations"},
			{Label: i18n.T(locale, "nav.roles"), Path: "/roles"},
			{Label: i18n.T(locale, "common.dashboard"), Path: "/"},
		},
	})
}

func (h *Handler) unit(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	h.renderArea(w, r, "/unit", templates.AreaPageData{
		Title:   i18n.T(locale, "area.unit_title"),
		Message: i18n.T(locale, "area.unit_message"),
		Links:   []templates.DashboardLink{{Label: i18n.T(locale, "common.dashboard"), Path: "/"}},
	})
}

func (h *Handler) tenancy(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	h.renderArea(w, r, "/tenancy", templates.AreaPageData{
		Title:   i18n.T(locale, "area.tenancy_title"),
		Message: i18n.T(locale, "area.tenancy_message"),
		Links:   []templates.DashboardLink{{Label: i18n.T(locale, "common.dashboard"), Path: "/"}},
	})
}

// renderArea renders an area page with the app shell, marking active as the
// current sidebar entry.
func (h *Handler) renderArea(w http.ResponseWriter, r *http.Request, active string, data templates.AreaPageData) {
	shell, err := httpx.ShellData(r, h.deps.Roles, active)
	if err != nil {
		http.Error(w, i18n.T(i18n.LanguageFrom(r.Context()), "home.error.load_session"), http.StatusInternalServerError)
		return
	}
	data.Shell = shell
	data.Locale = i18n.LanguageFrom(r.Context())
	httpx.Render(w, r, templates.AreaPage(data))
}
