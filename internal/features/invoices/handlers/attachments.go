package handlers

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/leoarkiteto/zelo/internal/features/invoices/services"
	"github.com/leoarkiteto/zelo/internal/shared/uploads"
)

// multipartMemory is the in-memory part of a parsed upload; the rest spills to
// temporary files. The total body size is capped by the BodyLimit middleware.
const multipartMemory = 32 << 20

// attachmentsPOST uploads one or more files (multipart field "files", FR-25).
func (h *Handler) attachmentsPOST(w http.ResponseWriter, r *http.Request) {
	user, condo := actor(r)
	id := r.PathValue("id")
	inv, err := h.deps.Invoices.Get(r.Context(), condo, id)
	if err != nil {
		h.loadError(w, r, err)
		return
	}

	if r.MultipartForm == nil {
		if err := r.ParseMultipartForm(multipartMemory); err != nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				fail(w, r, http.StatusRequestEntityTooLarge, "invoices.error.request_too_large")
				return
			}
			h.renderView(w, r, http.StatusUnprocessableEntity, inv, viewState{AttachError: tr(r, "invoices.error.invalid_file")})
			return
		}
	}
	var files = r.MultipartForm.File["files"]

	_, err = h.deps.Invoices.AddAttachments(r.Context(), user, condo, id, files)
	switch {
	case err == nil:
		http.Redirect(w, r, invoicePath(id)+"?flash=attachment_added#attachments", http.StatusSeeOther)
	case isNotFound(err):
		notFound(w, r)
	case errors.Is(err, services.ErrNoFiles):
		h.renderView(w, r, http.StatusUnprocessableEntity, inv, viewState{AttachError: tr(r, "invoices.error.no_files")})
	case errors.Is(err, services.ErrInvalidFile):
		h.renderView(w, r, http.StatusUnprocessableEntity, inv, viewState{AttachError: tr(r, "invoices.error.invalid_file")})
	case ruleKey(err) != "":
		h.renderView(w, r, http.StatusUnprocessableEntity, inv, viewState{AttachError: tr(r, ruleKey(err))})
	default:
		serverError(w, r, "add attachments", err, "invoices.error.save_failed")
	}
}

// attachmentGET downloads an attachment. The service resolves it inside the
// session's condominium and rejects path traversal (FR-26).
func (h *Handler) attachmentGET(w http.ResponseWriter, r *http.Request) {
	_, condo := actor(r)
	f, att, err := h.deps.Invoices.OpenAttachment(r.Context(), condo, r.PathValue("id"), r.PathValue("aid"))
	if err != nil {
		h.loadError(w, r, err)
		return
	}
	defer f.Close()

	ct := att.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": uploads.SanitizeFilename(att.OriginalName)})
	if disposition == "" {
		disposition = "attachment"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if st, err := f.Stat(); err == nil {
		w.Header().Set("Content-Length", strconv.FormatInt(st.Size(), 10))
	}
	_, _ = io.Copy(w, f)
}

// attachmentDeletePOST removes an attachment.
func (h *Handler) attachmentDeletePOST(w http.ResponseWriter, r *http.Request) {
	user, condo := actor(r)
	id := r.PathValue("id")
	err := h.deps.Invoices.RemoveAttachment(r.Context(), user, condo, id, r.PathValue("aid"))
	switch {
	case err == nil:
		http.Redirect(w, r, invoicePath(id)+"?flash=attachment_removed#attachments", http.StatusSeeOther)
	case isNotFound(err):
		notFound(w, r)
	case ruleKey(err) != "":
		inv, gerr := h.deps.Invoices.Get(r.Context(), condo, id)
		if gerr != nil {
			h.loadError(w, r, gerr)
			return
		}
		h.renderView(w, r, http.StatusUnprocessableEntity, inv, viewState{AttachError: tr(r, ruleKey(err))})
	default:
		serverError(w, r, "remove attachment", err, "invoices.error.save_failed")
	}
}
