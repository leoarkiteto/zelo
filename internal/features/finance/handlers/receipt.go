package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

const maxReceiptBytes = 5 << 20 // 5 MB

// errReceiptInvalid is returned when a receipt file fails type or size checks.
var errReceiptInvalid = errors.New("invalid receipt")

// receiptExtByType maps sniffed MIME types to file extensions.
var receiptExtByType = map[string]string{
	"application/pdf": "pdf",
	"image/jpeg":      "jpg",
	"image/png":       "png",
}

// saveReceipt stores an uploaded receipt under UploadDir and returns the
// stored relative filename and the original filename. A missing file returns
// empty values with no error. The stored extension is derived from the
// sniffed content (http.DetectContentType), never from the client's header.
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
	defer file.Close()

	if header.Size > maxReceiptBytes {
		return "", "", errReceiptInvalid
	}
	// Sniff the first 512 bytes to validate the real content type.
	head := make([]byte, 512)
	n, readErr := io.ReadFull(file, head)
	if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
		return "", "", fmt.Errorf("read receipt head: %w", readErr)
	}
	ext, ok := receiptExtByType[http.DetectContentType(head[:n])]
	if !ok {
		return "", "", errReceiptInvalid
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", "", fmt.Errorf("rewind receipt: %w", err)
	}

	if err := os.MkdirAll(h.deps.UploadDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create upload dir: %w", err)
	}
	randName, err := randomHex(16)
	if err != nil {
		return "", "", err
	}
	filename := randName + "." + ext
	dst, err := os.Create(filepath.Join(h.deps.UploadDir, filename))
	if err != nil {
		return "", "", fmt.Errorf("store receipt: %w", err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		return "", "", fmt.Errorf("copy receipt: %w", err)
	}
	return filename, header.Filename, nil
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
	base := filepath.Clean(h.deps.UploadDir)
	full := filepath.Clean(filepath.Join(base, account.ReceiptPath))
	if !strings.HasPrefix(full, base+string(os.PathSeparator)) {
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "finance.error.account_gone"))
		return
	}
	f, err := os.Open(full)
	if err != nil {
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "finance.error.account_gone"))
		return
	}
	defer f.Close()
	w.Header().Set("Content-Disposition",
		"attachment; filename=\""+sanitizeFilename(account.ReceiptName)+"\"")
	http.ServeContent(w, r, account.ReceiptName, account.UpdatedAt, f)
}

func sanitizeFilename(name string) string {
	if name == "" {
		return "receipt"
	}
	r := strings.NewReplacer(`"`, "", "\n", "", "\r", "")
	return r.Replace(name)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
