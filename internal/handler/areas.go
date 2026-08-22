package handler

import (
	"net/http"

	"github.com/leoarkiteto/zelo/web/templates"
)

func (h *Handler) condominium(w http.ResponseWriter, r *http.Request) {
	h.renderArea(w, r, "/condominium", templates.AreaPageData{
		Title:   "Condominium management",
		Message: "This is the syndic-only management area for the condominium.",
		Links: []templates.DashboardLink{
			{Label: "Invitations", Path: "/invitations"},
			{Label: "Role management", Path: "/roles"},
			{Label: "Dashboard", Path: "/"},
		},
	})
}

func (h *Handler) unit(w http.ResponseWriter, r *http.Request) {
	h.renderArea(w, r, "/unit", templates.AreaPageData{
		Title:   "My unit",
		Message: "Manage your own unit information here.",
		Links:   []templates.DashboardLink{{Label: "Dashboard", Path: "/"}},
	})
}

func (h *Handler) tenancy(w http.ResponseWriter, r *http.Request) {
	h.renderArea(w, r, "/tenancy", templates.AreaPageData{
		Title:   "My tenancy",
		Message: "View your own tenancy information here.",
		Links:   []templates.DashboardLink{{Label: "Dashboard", Path: "/"}},
	})
}

// renderArea renders an area page with the app shell, marking active as the
// current sidebar entry.
func (h *Handler) renderArea(w http.ResponseWriter, r *http.Request, active string, data templates.AreaPageData) {
	shell, err := h.shellData(r, active)
	if err != nil {
		http.Error(w, "Failed to load session", http.StatusInternalServerError)
		return
	}
	data.Shell = shell
	render(w, r, templates.AreaPage(data))
}
