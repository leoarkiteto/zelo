package handler

import (
	"net/http"

	"github.com/leoarkiteto/zelo/web/templates"
)

func (h *Handler) condominium(w http.ResponseWriter, r *http.Request) {
	render(w, r, templates.AreaPage(templates.AreaPageData{
		Title:   "Condominium management",
		Message: "This is the syndic-only management area for the condominium.",
		Links: []templates.DashboardLink{
			{Label: "Invitations", Path: "/invitations"},
			{Label: "Role management", Path: "/roles"},
			{Label: "Dashboard", Path: "/"},
		},
	}))
}

func (h *Handler) unit(w http.ResponseWriter, r *http.Request) {
	render(w, r, templates.AreaPage(templates.AreaPageData{
		Title:   "My unit",
		Message: "Manage your own unit information here.",
		Links:   []templates.DashboardLink{{Label: "Dashboard", Path: "/"}},
	}))
}

func (h *Handler) tenancy(w http.ResponseWriter, r *http.Request) {
	render(w, r, templates.AreaPage(templates.AreaPageData{
		Title:   "My tenancy",
		Message: "View your own tenancy information here.",
		Links:   []templates.DashboardLink{{Label: "Dashboard", Path: "/"}},
	}))
}
