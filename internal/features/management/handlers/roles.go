package handlers

import (
	"errors"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"net/http"

	"github.com/leoarkiteto/zelo/internal/features/management/core/services"
	"github.com/leoarkiteto/zelo/internal/features/management/templates"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

func (h *Handler) rolesGET(w http.ResponseWriter, r *http.Request) {
	data, err := h.rolesData(r, "")
	if err != nil {
		http.Error(w, "Failed to load roles", http.StatusInternalServerError)
		return
	}
	httpx.Render(w, r, templates.RolesPage(data))
}

func (h *Handler) rolesAssign(w http.ResponseWriter, r *http.Request) {
	h.changeRole(w, r, true)
}

func (h *Handler) rolesRevoke(w http.ResponseWriter, r *http.Request) {
	h.changeRole(w, r, false)
}

func (h *Handler) changeRole(w http.ResponseWriter, r *http.Request, grant bool) {
	sess := httpx.CurrentSession(r)
	caller := httpx.CurrentUser(r)
	targetID := r.FormValue("user_id")
	role := model.Role(r.FormValue("role"))
	if !role.Valid() {
		h.renderRolesWithError(w, r, "Unknown role.")
		return
	}
	var err error
	if grant {
		err = h.deps.RoleService.GrantRole(r.Context(), caller.ID, targetID, sess.CondominiumID, role)
	} else {
		err = h.deps.RoleService.RevokeRole(r.Context(), caller.ID, targetID, sess.CondominiumID, role)
	}
	switch {
	case errors.Is(err, services.ErrNotSyndic):
		h.renderRolesWithError(w, r, "Only the syndic can manage roles.")
	case errors.Is(err, services.ErrTargetNotOwner):
		h.renderRolesWithError(w, r, "The syndic role can only be granted to an owner.")
	case errors.Is(err, services.ErrTenantNotEligible):
		h.renderRolesWithError(w, r, "A tenant cannot be the syndic.")
	case err != nil:
		http.Error(w, "Failed to change role", http.StatusInternalServerError)
	default:
		http.Redirect(w, r, "/roles", http.StatusSeeOther)
	}
}

func (h *Handler) renderRolesWithError(w http.ResponseWriter, r *http.Request, message string) {
	data, err := h.rolesData(r, message)
	if err != nil {
		http.Error(w, "Failed to load roles", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	httpx.Render(w, r, templates.RolesPage(data))
}

func (h *Handler) rolesData(r *http.Request, message string) (templates.RolesPageData, error) {
	sess := httpx.CurrentSession(r)
	shell, err := httpx.ShellData(r, h.deps.Roles, "/roles")
	if err != nil {
		return templates.RolesPageData{}, err
	}
	rows, err := h.deps.Roles.ListUsersWithRoles(r.Context(), sess.CondominiumID)
	if err != nil {
		return templates.RolesPageData{}, err
	}
	views := make([]templates.UserRoleView, 0, len(rows))
	for _, row := range rows {
		roles := make([]string, 0, len(row.Roles))
		for _, role := range row.Roles {
			roles = append(roles, string(role))
		}
		views = append(views, templates.UserRoleView{
			UserID: row.UserID,
			Email:  row.Email,
			Roles:  roles,
		})
	}
	return templates.RolesPageData{Shell: shell, CSRF: sess.CSRFToken, Users: views, Error: message}, nil
}
