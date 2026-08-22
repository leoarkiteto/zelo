package handler

import (
	"net/http"
	"strings"
	"time"

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

	shell, err := h.shellDataWithRoles(r, "/", roles)
	if err != nil {
		http.Error(w, "Failed to load session", http.StatusInternalServerError)
		return
	}

	has := func(role model.Role) bool {
		for _, have := range roles {
			if have == role {
				return true
			}
		}
		return false
	}
	var actions []templates.QuickAction
	if has(model.RoleSyndic) {
		actions = append(actions,
			templates.QuickAction{Icon: "building", Label: "Condominium management", Description: "Oversee condominium settings and governance.", Path: "/condominium"},
			templates.QuickAction{Icon: "mail", Label: "Invitations", Description: "Invite owners and tenants to join.", Path: "/invitations"},
			templates.QuickAction{Icon: "users", Label: "Role management", Description: "Assign and revoke roles within the condominium.", Path: "/roles"},
		)
	}
	if has(model.RoleOwner) || has(model.RoleSyndic) {
		actions = append(actions, templates.QuickAction{Icon: "home", Label: "My unit", Description: "View and manage your unit information.", Path: "/unit"})
	}
	if has(model.RoleTenant) {
		actions = append(actions, templates.QuickAction{Icon: "key", Label: "My tenancy", Description: "View your tenancy details.", Path: "/tenancy"})
	}

	render(w, r, templates.DashboardPage(templates.DashboardData{
		Shell:        shell,
		Greeting:     greetingFor(time.Now()),
		Name:         displayName(u.Email),
		Date:         time.Now().Format("Monday, January 2, 2006"),
		Roles:        roleStrs,
		QuickActions: actions,
	}))
}

// greetingFor returns the time-of-day greeting used on the dashboard.
func greetingFor(t time.Time) string {
	switch h := t.Hour(); {
	case h < 12:
		return "Good morning"
	case h < 18:
		return "Good afternoon"
	default:
		return "Good evening"
	}
}

// displayName derives a capitalized display name from the email local part.
func displayName(email string) string {
	local := strings.Split(email, "@")[0]
	if local == "" {
		return "there"
	}
	return strings.ToUpper(local[:1]) + local[1:]
}
