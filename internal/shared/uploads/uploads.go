// Package uploads stores and serves user-uploaded attachments (PDF, JPEG, PNG).
//
// Subdirectories: every function takes the storage directory as dir and works
// with paths relative to it. A feature that wants its own subdirectory passes
// filepath.Join(uploadDir, "invoices") as dir; Stored.Path is then relative to
// that directory, and the same dir must be passed to Open and Remove. Features
// that predate this package (finance) keep passing UPLOAD_DIR itself, so their
// existing flat files stay reachable.
package uploads

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ErrInvalid is returned when a file fails the type, size or path checks.
var ErrInvalid = errors.New("invalid upload")

// extByType maps sniffed MIME types to file extensions.
var extByType = map[string]string{
	"application/pdf": "pdf",
	"image/jpeg":      "jpg",
	"image/png":       "png",
}

// Stored describes a file written by Save.
type Stored struct {
	Path         string // relative to the dir passed to Save: random name + sniffed extension
	OriginalName string // client-supplied filename (untrusted; use SanitizeFilename for headers)
	ContentType  string // sniffed MIME type
	Size         int64
}

// Save validates and stores an uploaded file under dir. The type is sniffed
// from the content (http.DetectContentType), never taken from the client, and
// the stored name is 32 random hex characters plus the sniffed extension.
// Files larger than maxBytes, or of another type, return ErrInvalid.
func Save(dir string, fh *multipart.FileHeader, maxBytes int64) (Stored, error) {
	if fh == nil {
		return Stored{}, ErrInvalid
	}
	if fh.Size > maxBytes {
		return Stored{}, ErrInvalid
	}
	file, err := fh.Open()
	if err != nil {
		return Stored{}, fmt.Errorf("open upload: %w", err)
	}
	defer file.Close()

	head := make([]byte, 512)
	n, readErr := io.ReadFull(file, head)
	if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
		return Stored{}, fmt.Errorf("read upload head: %w", readErr)
	}
	ctype := http.DetectContentType(head[:n])
	ext, ok := extByType[ctype]
	if !ok {
		return Stored{}, ErrInvalid
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return Stored{}, fmt.Errorf("rewind upload: %w", err)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Stored{}, fmt.Errorf("create upload dir: %w", err)
	}
	randName, err := randomHex(16)
	if err != nil {
		return Stored{}, err
	}
	filename := randName + "." + ext
	dst, err := os.OpenFile(filepath.Join(dir, filename), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return Stored{}, fmt.Errorf("store upload: %w", err)
	}
	// Copy at most maxBytes+1 so a header lying about Size cannot exceed the limit.
	written, err := io.Copy(dst, io.LimitReader(file, maxBytes+1))
	if cerr := dst.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(filepath.Join(dir, filename))
		return Stored{}, fmt.Errorf("copy upload: %w", err)
	}
	if written > maxBytes {
		_ = os.Remove(filepath.Join(dir, filename))
		return Stored{}, ErrInvalid
	}
	return Stored{Path: filename, OriginalName: fh.Filename, ContentType: ctype, Size: written}, nil
}

// resolve joins rel to dir and rejects anything that escapes dir.
func resolve(dir, rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) || strings.ContainsRune(rel, 0) {
		return "", ErrInvalid
	}
	base := filepath.Clean(dir)
	full := filepath.Clean(filepath.Join(base, rel))
	if !strings.HasPrefix(full, base+string(os.PathSeparator)) {
		return "", ErrInvalid
	}
	return full, nil
}

// Open opens a stored file for reading. Paths that are empty, absolute or
// escape dir return ErrInvalid.
func Open(dir, rel string) (*os.File, error) {
	full, err := resolve(dir, rel)
	if err != nil {
		return nil, err
	}
	return os.Open(full)
}

// Remove deletes a stored file. A file that is already gone is not an error.
func Remove(dir, rel string) error {
	full, err := resolve(dir, rel)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// SanitizeFilename makes a client filename safe for a Content-Disposition
// header. An empty name becomes fallback.
func SanitizeFilename(name, fallback string) string {
	if name == "" {
		return fallback
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
