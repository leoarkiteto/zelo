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
// the entry matching active as current. Related entries are grouped into
// collapsible submenus (Finance, Tickets, Condominium); a group left with a
// single visible entry is flattened to a plain link, so small menus stay flat.
// Labels come from the message catalog.
func NavFor(roles []model.Role, active string, locale i18n.Language) []organisms.NavItem {
	has := func(role model.Role) bool {
		for _, have := range roles {
			if have == role {
				return true
			}
		}
		return false
	}
	syndic := has(model.RoleSyndic)
	link := func(key i18n.MessageKey, path string) organisms.NavItem {
		return organisms.NavItem{Label: i18n.T(locale, key), Path: path}
	}
	group := func(key i18n.MessageKey, children ...organisms.NavItem) organisms.NavItem {
		return organisms.NavItem{Label: i18n.T(locale, key), Children: children}
	}

	finance := group("nav.group.finance", link("nav.finance.my_charges", "/finance/my-charges"))
	tickets := group("nav.group.tickets", link("nav.tickets", "/tickets"))
	condominium := group("nav.group.condominium")
	if syndic {
		finance.Children = append(finance.Children,
			link("nav.finance.overview", "/finance"),
			link("nav.invoices", "/invoices"),
			link("nav.suppliers", "/suppliers"),
		)
		tickets.Children = append(tickets.Children, link("nav.tickets.inbox", "/tickets/inbox"))
		condominium.Children = append(condominium.Children,
			link("nav.management", "/condominium"),
			link("nav.condominium_settings", "/condominium/settings"),
			link("nav.units", "/condominium/units"),
			link("nav.invitations", "/invitations"),
			link("nav.roles", "/roles"),
		)
	}

	items := []organisms.NavItem{
		link("nav.dashboard", "/"),
		link("nav.directory", "/directory"),
		finance,
		tickets,
		condominium,
	}
	if has(model.RoleOwner) || syndic {
		items = append(items, link("nav.unit", "/unit"))
	}
	if has(model.RoleTenant) {
		items = append(items, link("nav.tenancy", "/tenancy"))
	}
	return markActive(compactNav(items), active)
}

// compactNav drops empty groups and replaces single-entry groups with their
// only entry, keeping the original order.
func compactNav(items []organisms.NavItem) []organisms.NavItem {
	out := make([]organisms.NavItem, 0, len(items))
	for _, it := range items {
		if it.Path == "" && len(it.Children) <= 1 {
			out = append(out, it.Children...)
			continue
		}
		out = append(out, it)
	}
	return out
}

// markActive flags the entry whose path equals active, and every group that
// contains it so the group renders open.
func markActive(items []organisms.NavItem, active string) []organisms.NavItem {
	for i := range items {
		if len(items[i].Children) > 0 {
			items[i].Children = markActive(items[i].Children, active)
			items[i].Active = false
			for _, c := range items[i].Children {
				items[i].Active = items[i].Active || c.Active
			}
			continue
		}
		items[i].Active = items[i].Path == active
	}
	return items
}
