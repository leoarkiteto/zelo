package handlers

import (
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"net/http"
	"strings"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/management/templates"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/security"
)

func (h *Handler) invitationsGET(w http.ResponseWriter, r *http.Request) {
	data, err := h.invitationsData(r, "")
	if err != nil {
		http.Error(w, "Failed to load invitations", http.StatusInternalServerError)
		return
	}
	httpx.Render(w, r, templates.InvitationsPage(data))
}

func (h *Handler) invitationsPOST(w http.ResponseWriter, r *http.Request) {
	sess := httpx.CurrentSession(r)
	u := httpx.CurrentUser(r)

	role := model.Role(r.FormValue("invited_role"))
	if role != model.RoleOwner && role != model.RoleTenant {
		h.renderInvitationsWithError(w, r, "Invited role must be owner or tenant.")
		return
	}
	rawToken, err := security.NewCSRFToken()
	if err != nil {
		http.Error(w, "Failed to create invitation", http.StatusInternalServerError)
		return
	}
	inv := model.Invitation{
		TokenHash:     h.deps.Tokens.HashToken(rawToken),
		CondominiumID: sess.CondominiumID,
		UnitID:        r.FormValue("unit_id"),
		InvitedRole:   role,
		InvitedEmail:  strings.TrimSpace(r.FormValue("invited_email")),
		ExpiresAt:     time.Now().Add(7 * 24 * time.Hour),
		CreatedBy:     u.ID,
	}
	if _, err := h.deps.Invitations.CreateInvitation(r.Context(), inv); err != nil {
		http.Error(w, "Failed to create invitation", http.StatusInternalServerError)
		return
	}
	data, err := h.invitationsData(r, "Invitation created. Share this link with the invitee: /register?token="+rawToken)
	if err != nil {
		http.Error(w, "Failed to load invitations", http.StatusInternalServerError)
		return
	}
	httpx.Render(w, r, templates.InvitationsPage(data))
}

func (h *Handler) invitationsRevoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.deps.Invitations.RevokeInvitation(r.Context(), id); err != nil {
		http.Error(w, "Failed to revoke invitation", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/invitations", http.StatusSeeOther)
}

func (h *Handler) renderInvitationsWithError(w http.ResponseWriter, r *http.Request, message string) {
	data, err := h.invitationsData(r, "")
	if err != nil {
		http.Error(w, "Failed to load invitations", http.StatusInternalServerError)
		return
	}
	data.Error = message
	w.WriteHeader(http.StatusBadRequest)
	httpx.Render(w, r, templates.InvitationsPage(data))
}

func (h *Handler) invitationsData(r *http.Request, flash string) (templates.InvitationsPageData, error) {
	sess := httpx.CurrentSession(r)
	shell, err := httpx.ShellData(r, h.deps.Roles, "/invitations")
	if err != nil {
		return templates.InvitationsPageData{}, err
	}
	units, err := h.deps.Units.ListUnitsForCondominium(r.Context(), sess.CondominiumID)
	if err != nil {
		return templates.InvitationsPageData{}, err
	}
	invitations, err := h.deps.Invitations.ListInvitationsForCondominium(r.Context(), sess.CondominiumID)
	if err != nil {
		return templates.InvitationsPageData{}, err
	}
	unitCode := map[string]string{}
	for _, unit := range units {
		unitCode[unit.ID] = unit.Code
	}
	unitOptions := make([]templates.UnitOption, 0, len(units))
	for _, unit := range units {
		unitOptions = append(unitOptions, templates.UnitOption{ID: unit.ID, Code: unit.Code})
	}
	views := make([]templates.InvitationView, 0, len(invitations))
	for _, inv := range invitations {
		views = append(views, templates.InvitationView{
			ID:        inv.ID,
			UnitCode:  unitCode[inv.UnitID],
			Role:      string(inv.InvitedRole),
			Email:     inv.InvitedEmail,
			Status:    string(inv.Status),
			ExpiresAt: inv.ExpiresAt.Format(time.RFC3339),
		})
	}
	return templates.InvitationsPageData{
		Shell:       shell,
		CSRF:        sess.CSRFToken,
		Units:       unitOptions,
		Invitations: views,
		Flash:       flash,
	}, nil
}
