package handler

import (
	"net/http"

	"github.com/leoarkiteto/zelo/internal/model"
	"github.com/leoarkiteto/zelo/web/templates"
)

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	sess := currentSession(r)
	roles, err := h.deps.Roles.ActiveRolesForUser(r.Context(), u.ID, sess.CondominiumID)
	if err != nil {
		http.Error(w, "Failed to load roles", http.StatusInternalServerError)
		return
	}
	roleStrs := make([]string, 0, len(roles))
	for _, role := range roles {
		roleStrs = append(roleStrs, string(role))
	}

	var links []templates.DashboardLink
	has := func(role model.Role) bool {
		for _, have := range roles {
			if have == role {
				return true
			}
		}
		return false
	}
	if has(model.RoleSyndic) {
		links = append(links,
			templates.DashboardLink{Label: "Condominium management", Path: "/condominium"},
			templates.DashboardLink{Label: "Invitations", Path: "/invitations"},
			templates.DashboardLink{Label: "Role management", Path: "/roles"},
		)
	}
	if has(model.RoleOwner) || has(model.RoleSyndic) {
		links = append(links, templates.DashboardLink{Label: "My unit", Path: "/unit"})
	}
	if has(model.RoleTenant) {
		links = append(links, templates.DashboardLink{Label: "My tenancy", Path: "/tenancy"})
	}
	render(w, r, templates.DashboardPage(templates.DashboardData{Email: u.Email, Roles: roleStrs, Links: links}))
}
