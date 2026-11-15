package middleware

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/model"
)

type fakeSessions struct{ sess model.Session }

func (f fakeSessions) Read(*http.Request) (model.Session, error) { return f.sess, nil }

// bodyLimitChain builds BodyLimit -> CSRF -> ok, as wired in main.go.
func bodyLimitChain() http.Handler {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	var h http.Handler = ok
	h = CSRF(fakeSessions{model.Session{TokenHash: "t", UserID: "u", CSRFToken: "tok-123"}})(h)
	return BodyLimit(1<<20, []BodyLimitRule{
		{Method: http.MethodPost, Segments: []string{"finance"}, Bytes: 6 << 20},
		{Method: http.MethodPost, Segments: []string{"finance", "*", "edit"}, Bytes: 6 << 20},
		{Method: http.MethodPost, Segments: []string{"invoices", "*", "attachments"}, Bytes: 52 << 20},
	})(h)
}

func multipartReq(t *testing.T, path string, size int) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("csrf_token", "tok-123")
	fw, err := mw.CreateFormFile("receipt", "r.pdf")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(bytes.Repeat([]byte("a"), size))
	_ = mw.Close()
	r := httptest.NewRequest(http.MethodPost, path, &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r
}

func TestBodyLimitMultipart(t *testing.T) {
	tests := []struct {
		name string
		path string
		size int
		want int
	}{
		{"2MB finance ok", "/finance", 2 << 20, http.StatusOK},
		{"7MB finance too large", "/finance", 7 << 20, http.StatusRequestEntityTooLarge},
		{"2MB finance edit ok", "/finance/abc/edit", 2 << 20, http.StatusOK},
		{"7MB finance edit too large", "/finance/abc/edit", 7 << 20, http.StatusRequestEntityTooLarge},
		{"2MB other path too large", "/finance/abc/settle", 2 << 20, http.StatusRequestEntityTooLarge},
		{"2MB attachments ok", "/invoices/abc/attachments", 2 << 20, http.StatusOK},
		{"small other path ok", "/profile", 1000, http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			bodyLimitChain().ServeHTTP(rec, multipartReq(t, tc.path, tc.size))
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestBodyLimitURLEncodedTooLarge(t *testing.T) {
	body := "csrf_token=tok-123&x=" + strings.Repeat("a", 2<<20)
	r := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	bodyLimitChain().ServeHTTP(rec, r)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

func TestCSRFInvalidTokenStill403(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader("csrf_token=bad"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	bodyLimitChain().ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestMatchSegments(t *testing.T) {
	if !matchSegments([]string{"a", "*", "c"}, []string{"a", "b", "c"}) {
		t.Fatal("wildcard should match one segment")
	}
	if matchSegments([]string{"a", "*"}, []string{"a", "b", "c"}) || matchSegments([]string{"a", "*"}, []string{"a"}) {
		t.Fatal("wildcard must match exactly one segment")
	}
}
