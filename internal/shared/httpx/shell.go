package httpx

import (
	"context"
	"net/http"
	"strings"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/templates/organisms"
)

// RoleReader returns the active roles for a user in a condominium.
type RoleReader interface {
	ActiveRolesForUser(ctx context.Context, userID, condominiumID string) ([]model.Role, error)
}

// ShellData builds the authenticated app-shell context (topbar identity,
// role-based sidebar navigation, CSRF token, resolved locale) for the given
// active path.
func ShellData(r *http.Request, roles RoleReader, active string) (*organisms.ShellData, error) {
	rolesList, err := roles.ActiveRolesForUser(r.Context(), CurrentUser(r).ID, CurrentSession(r).CondominiumID)
	if err != nil {
		return nil, err
	}
	return ShellDataWithRoles(r, active, rolesList)
}

// ShellDataWithRoles is ShellData for callers that already loaded the user's
// active roles.
func ShellDataWithRoles(r *http.Request, active string, roles []model.Role) (*organisms.ShellData, error) {
	u := CurrentUser(r)
	locale := i18n.LanguageFrom(r.Context())
	roleStrs := make([]string, 0, len(roles))
	for _, role := range roles {
		roleStrs = append(roleStrs, string(role))
	}
	initial := ""
	if u.Email != "" {
		initial = strings.ToUpper(u.Email[:1])
	}
	return &organisms.ShellData{
		User:   &organisms.UserView{Email: u.Email, Roles: roleStrs, Initial: initial},
		Nav:    NavFor(roles, active, locale),
		CSRF:   CurrentSession(r).CSRFToken,
		Locale: locale,
	}, nil
}

// NavFor derives the sidebar navigation from the user's active roles, marking
// the entry matching active as current. Labels come from the message catalog.
func NavFor(roles []model.Role, active string, locale i18n.Language) []organisms.NavItem {
	has := func(role model.Role) bool {
		for _, have := range roles {
			if have == role {
				return true
			}
		}
		return false
	}
	items := []organisms.NavItem{
		{Label: i18n.T(locale, "nav.dashboard"), Path: "/"},
		{Label: i18n.T(locale, "nav.directory"), Path: "/directory"},
		{Label: i18n.T(locale, "nav.tickets"), Path: "/tickets"},
		{Label: i18n.T(locale, "nav.finance.my_charges"), Path: "/finance/my-charges"},
		{Label: i18n.T(locale, "nav.finance.health"), Path: "/finance/health"},
	}
	if has(model.RoleSyndic) {
		items = append(items,
			organisms.NavItem{Label: i18n.T(locale, "nav.finance"), Path: "/finance"},
			organisms.NavItem{Label: i18n.T(locale, "nav.tickets.inbox"), Path: "/tickets/inbox"},
			organisms.NavItem{Label: i18n.T(locale, "nav.management"), Path: "/condominium"},
			organisms.NavItem{Label: i18n.T(locale, "nav.invitations"), Path: "/invitations"},
			organisms.NavItem{Label: i18n.T(locale, "nav.roles"), Path: "/roles"},
		)
	}
	if has(model.RoleOwner) || has(model.RoleSyndic) {
		items = append(items, organisms.NavItem{Label: i18n.T(locale, "nav.unit"), Path: "/unit"})
	}
	if has(model.RoleTenant) {
		items = append(items, organisms.NavItem{Label: i18n.T(locale, "nav.tenancy"), Path: "/tenancy"})
	}
	for i := range items {
		items[i].Active = items[i].Path == active
	}
	return items
}
