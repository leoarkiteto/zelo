package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/uploads"
)

const maxReceiptBytes = 5 << 20 // 5 MB

// errReceiptInvalid is returned when a receipt file fails type or size checks.
var errReceiptInvalid = uploads.ErrInvalid

// saveReceipt stores an uploaded receipt flat under UploadDir and returns the
// stored relative filename and the original filename. A missing file returns
// empty values with no error.
func (h *Handler) saveReceipt(r *http.Request) (path, name string, err error) {
	// Plain url-encoded forms (no file input attached) have no receipt.
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return "", "", nil
	}
	file, header, ferr := r.FormFile("receipt")
	if errors.Is(ferr, http.ErrMissingFile) {
		return "", "", nil
	}
	if ferr != nil {
		return "", "", fmt.Errorf("read receipt upload: %w", ferr)
	}
	file.Close()

	st, err := uploads.Save(h.deps.UploadDir, header, maxReceiptBytes)
	if err != nil {
		return "", "", err
	}
	return st.Path, st.OriginalName, nil
}

// financeReceiptGET streams the stored comprovante to the syndic (FR-017:
// receipts are manager-only).
func (h *Handler) financeReceiptGET(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	account, err := h.accountForCondominium(r, r.PathValue("id"))
	if err != nil {
		h.renderAccountGone(w, r, err)
		return
	}
	if account.ReceiptPath == "" {
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "finance.error.account_gone"))
		return
	}
	f, err := uploads.Open(h.deps.UploadDir, account.ReceiptPath)
	if err != nil {
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "finance.error.account_gone"))
		return
	}
	defer f.Close()
	w.Header().Set("Content-Disposition",
		"attachment; filename=\""+uploads.SanitizeFilename(account.ReceiptName)+"\"")
	http.ServeContent(w, r, account.ReceiptName, account.UpdatedAt, f)
}
