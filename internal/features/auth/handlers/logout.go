package handlers

import (
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"net/http"

	"github.com/leoarkiteto/zelo/internal/shared/model"
)

func (h *Handler) logoutPOST(w http.ResponseWriter, r *http.Request) {
	if u := httpx.CurrentUser(r); u != nil {
		uid := u.ID
		_ = h.deps.Audit.RecordEvent(r.Context(), &uid, model.AuditSignOut, map[string]any{"path": "/logout"})
	}
	h.deps.Sessions.Invalidate(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
