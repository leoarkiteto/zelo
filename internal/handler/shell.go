package handler

import (
	"net/http"
	"strings"

	"github.com/leoarkiteto/zelo/internal/model"
	"github.com/leoarkiteto/zelo/web/templates"
)

// shellData builds the authenticated app-shell context (topbar identity,
// role-based sidebar navigation, CSRF token) for the given active path.
func (h *Handler) shellData(r *http.Request, active string) (*templates.ShellData, error) {
	roles, err := h.deps.Roles.ActiveRolesForUser(r.Context(), currentUser(r).ID, currentSession(r).CondominiumID)
	if err != nil {
		return nil, err
	}
	return h.shellDataWithRoles(r, active, roles)
}

// shellDataWithRoles is shellData for callers that already loaded the user's
// active roles.
func (h *Handler) shellDataWithRoles(r *http.Request, active string, roles []model.Role) (*templates.ShellData, error) {
	u := currentUser(r)
	roleStrs := make([]string, 0, len(roles))
	for _, role := range roles {
		roleStrs = append(roleStrs, string(role))
	}
	initial := ""
	if u.Email != "" {
		initial = strings.ToUpper(u.Email[:1])
	}
	return &templates.ShellData{
		User: &templates.UserView{Email: u.Email, Roles: roleStrs, Initial: initial},
		Nav:  navFor(roles, active),
		CSRF: currentSession(r).CSRFToken,
	}, nil
}

// navFor derives the sidebar navigation from the user's active roles, marking
// the entry matching active as current.
func navFor(roles []model.Role, active string) []templates.NavItem {
	has := func(role model.Role) bool {
		for _, have := range roles {
			if have == role {
				return true
			}
		}
		return false
	}
	items := []templates.NavItem{{Label: "Dashboard", Path: "/"}}
	if has(model.RoleSyndic) {
		items = append(items,
			templates.NavItem{Label: "Condominium management", Path: "/condominium"},
			templates.NavItem{Label: "Invitations", Path: "/invitations"},
			templates.NavItem{Label: "Role management", Path: "/roles"},
		)
	}
	if has(model.RoleOwner) || has(model.RoleSyndic) {
		items = append(items, templates.NavItem{Label: "My unit", Path: "/unit"})
	}
	if has(model.RoleTenant) {
		items = append(items, templates.NavItem{Label: "My tenancy", Path: "/tenancy"})
	}
	for i := range items {
		items[i].Active = items[i].Path == active
	}
	return items
}
