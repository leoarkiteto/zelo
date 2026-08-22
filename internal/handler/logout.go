package handler

import (
	"net/http"

	"github.com/leoarkiteto/zelo/internal/model"
)

func (h *Handler) logoutPOST(w http.ResponseWriter, r *http.Request) {
	if u := currentUser(r); u != nil {
		uid := u.ID
		_ = h.deps.Audit.RecordEvent(r.Context(), &uid, model.AuditSignOut, map[string]any{"path": "/logout"})
	}
	h.deps.Sessions.Invalidate(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
