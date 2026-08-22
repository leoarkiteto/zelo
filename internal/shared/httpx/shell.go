package httpx

import (
	"context"
	"net/http"
	"strings"

	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/templates"
)

// RoleReader returns the active roles for a user in a condominium.
type RoleReader interface {
	ActiveRolesForUser(ctx context.Context, userID, condominiumID string) ([]model.Role, error)
}

// ShellData builds the authenticated app-shell context (topbar identity,
// role-based sidebar navigation, CSRF token) for the given active path.
func ShellData(r *http.Request, roles RoleReader, active string) (*templates.ShellData, error) {
	rolesList, err := roles.ActiveRolesForUser(r.Context(), CurrentUser(r).ID, CurrentSession(r).CondominiumID)
	if err != nil {
		return nil, err
	}
	return ShellDataWithRoles(r, active, rolesList)
}

// ShellDataWithRoles is ShellData for callers that already loaded the user's
// active roles.
func ShellDataWithRoles(r *http.Request, active string, roles []model.Role) (*templates.ShellData, error) {
	u := CurrentUser(r)
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
		Nav:  NavFor(roles, active),
		CSRF: CurrentSession(r).CSRFToken,
	}, nil
}

// NavFor derives the sidebar navigation from the user's active roles, marking
// the entry matching active as current.
func NavFor(roles []model.Role, active string) []templates.NavItem {
	has := func(role model.Role) bool {
		for _, have := range roles {
			if have == role {
				return true
			}
		}
		return false
	}
	items := []templates.NavItem{{Label: "Dashboard", Path: "/"}, {Label: "Service directory", Path: "/directory"}}
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
