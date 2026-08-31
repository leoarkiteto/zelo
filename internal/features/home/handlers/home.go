package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/home/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())
	roles, err := h.deps.Roles.ActiveRolesForUser(r.Context(), u.ID, sess.CondominiumID)
	if err != nil {
		http.Error(
			w,
			i18n.T(i18n.LanguageFrom(r.Context()), "home.error.load_roles"),
			http.StatusInternalServerError,
		)
		return
	}
	roleStrs := make([]string, 0, len(roles))
	for _, role := range roles {
		roleStrs = append(roleStrs, string(role))
	}

	shell, err := httpx.ShellDataWithRoles(r, "/", roles)
	if err != nil {
		http.Error(
			w,
			i18n.T(i18n.LanguageFrom(r.Context()), "home.error.load_session"),
			http.StatusInternalServerError,
		)
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
		actions = append(
			actions,
			templates.QuickAction{
				Icon:        "building",
				Label:       i18n.T(locale, "home.action.condominium"),
				Description: i18n.T(locale, "home.action.condominium_desc"),
				Path:        "/condominium",
			},
			templates.QuickAction{
				Icon:        "mail",
				Label:       i18n.T(locale, "home.action.invitations"),
				Description: i18n.T(locale, "home.action.invitations_desc"),
				Path:        "/invitations",
			},
			templates.QuickAction{
				Icon:        "users",
				Label:       i18n.T(locale, "home.action.roles"),
				Description: i18n.T(locale, "home.action.roles_desc"),
				Path:        "/roles",
			},
		)
	}
	if has(model.RoleOwner) || has(model.RoleSyndic) {
		actions = append(
			actions,
			templates.QuickAction{
				Icon:        "home",
				Label:       i18n.T(locale, "home.action.unit"),
				Description: i18n.T(locale, "home.action.unit_desc"),
				Path:        "/unit",
			},
		)
	}
	if has(model.RoleTenant) {
		actions = append(
			actions,
			templates.QuickAction{
				Icon:        "key",
				Label:       i18n.T(locale, "home.action.tenancy"),
				Description: i18n.T(locale, "home.action.tenancy_desc"),
				Path:        "/tenancy",
			},
		)
	}

	httpx.Render(w, r, templates.DashboardPage(templates.DashboardData{
		Shell:        shell,
		Locale:       locale,
		Greeting:     greetingFor(time.Now(), locale),
		Name:         displayName(u.Email),
		Date:         i18n.FormatLongDate(locale, time.Now()),
		Roles:        roleStrs,
		QuickActions: actions,
	}))
}

// greetingFor returns the time-of-day greeting used on the dashboard.
func greetingFor(t time.Time, locale i18n.Language) string {
	switch h := t.Hour(); {
	case h < 12:
		return i18n.T(locale, "home.greeting.morning")
	case h < 18:
		return i18n.T(locale, "home.greeting.afternoon")
	default:
		return i18n.T(locale, "home.greeting.evening")
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
