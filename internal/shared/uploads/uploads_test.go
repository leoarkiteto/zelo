package uploads

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func header(t *testing.T, name string, data []byte) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	w, err := mw.CreateFormFile("f", name)
	if err != nil {
		t.Fatal(err)
	}
	w.Write(data)
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		t.Fatal(err)
	}
	return req.MultipartForm.File["f"][0]
}

func TestSaveAcceptsSniffedTypes(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var jb, pb bytes.Buffer
	jpeg.Encode(&jb, img, nil)
	png.Encode(&pb, img)
	cases := []struct {
		name, ctype, ext string
		data             []byte
	}{
		{"a.pdf", "application/pdf", ".pdf", []byte("%PDF-1.4\nhello")},
		{"b.bin", "image/jpeg", ".jpg", jb.Bytes()},
		{"c.txt", "image/png", ".png", pb.Bytes()},
	}
	re := regexp.MustCompile(`^[0-9a-f]{32}\.`)
	for _, c := range cases {
		dir := filepath.Join(t.TempDir(), "sub")
		st, err := Save(dir, header(t, c.name, c.data), 5<<20)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if st.ContentType != c.ctype || !strings.HasSuffix(st.Path, c.ext) || !re.MatchString(st.Path) {
			t.Errorf("%s: got %+v", c.name, st)
		}
		if st.OriginalName != c.name || st.Size != int64(len(c.data)) {
			t.Errorf("%s: name/size wrong: %+v", c.name, st)
		}
		f, err := Open(dir, st.Path)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(f)
		f.Close()
		if !bytes.Equal(got, c.data) {
			t.Errorf("%s: content mismatch", c.name)
		}
		if err := Remove(dir, st.Path); err != nil {
			t.Fatal(err)
		}
		if _, err := Open(dir, st.Path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("expected not-exist after Remove, got %v", err)
		}
	}
}

func TestSaveRejectsFakePDF(t *testing.T) {
	_, err := Save(t.TempDir(), header(t, "fake.pdf", []byte("just plain text")), 5<<20)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid, got %v", err)
	}
}

func TestSaveRejectsOversize(t *testing.T) {
	data := append([]byte("%PDF-1.4\n"), make([]byte, 2000)...)
	_, err := Save(t.TempDir(), header(t, "big.pdf", data), 1000)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid, got %v", err)
	}
}

func TestOpenRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(filepath.Dir(dir), "secret.txt")
	os.WriteFile(outside, []byte("x"), 0o644)
	defer os.Remove(outside)
	for _, rel := range []string{"../secret.txt", "../x", outside, "/etc/passwd", "", "a/../../secret.txt"} {
		if f, err := Open(dir, rel); !errors.Is(err, ErrInvalid) {
			if f != nil {
				f.Close()
			}
			t.Errorf("Open(%q) err = %v, want ErrInvalid", rel, err)
		}
		if err := Remove(dir, rel); !errors.Is(err, ErrInvalid) {
			t.Errorf("Remove(%q) err = %v, want ErrInvalid", rel, err)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	if got := SanitizeFilename(""); got != "receipt" {
		t.Errorf("empty: %q", got)
	}
	if got := SanitizeFilename("a\"b\r\nc.pdf"); got != "abc.pdf" {
		t.Errorf("got %q", got)
	}
}
