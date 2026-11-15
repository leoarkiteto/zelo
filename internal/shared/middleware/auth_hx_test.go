package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/model"
)

func TestRequireRoleHX(t *testing.T) {
	hxDenied := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("hx-denied-partial"))
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	build := func(roles ...model.Role) (http.Handler, *fakeAuditRecorder) {
		audit := &fakeAuditRecorder{}
		return RequireRoleHX(&fakeRoleChecker{roles: roles}, audit, hxDenied, model.RoleSyndic)(next), audit
	}

	t.Run("HX unauthenticated gets HX-Redirect", func(t *testing.T) {
		h, _ := build(model.RoleSyndic)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized || rec.Header().Get("HX-Redirect") != "/login" {
			t.Fatalf("status=%d HX-Redirect=%q", rec.Code, rec.Header().Get("HX-Redirect"))
		}
		if rec.Header().Get("Vary") != "HX-Request" {
			t.Errorf("Vary = %q", rec.Header().Get("Vary"))
		}
	})

	t.Run("HX denied uses hxDenied and audits", func(t *testing.T) {
		h, audit := build(model.RoleOwner)
		req := authenticatedRequest(true)
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || rec.Body.String() != "hx-denied-partial" {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
		if audit.denied != 1 {
			t.Errorf("denied audit = %d, want 1", audit.denied)
		}
		if rec.Header().Get("Vary") != "HX-Request" {
			t.Errorf("Vary = %q", rec.Header().Get("Vary"))
		}
	})

	t.Run("plain unauthenticated redirects to login", func(t *testing.T) {
		h, _ := build(model.RoleSyndic)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
			t.Fatalf("status=%d Location=%q", rec.Code, rec.Header().Get("Location"))
		}
		if rec.Header().Get("Vary") != "HX-Request" {
			t.Errorf("Vary = %q", rec.Header().Get("Vary"))
		}
	})

	t.Run("plain denied is standard 403", func(t *testing.T) {
		h, audit := build(model.RoleOwner)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, authenticatedRequest(true))
		if rec.Code != http.StatusForbidden || rec.Body.String() == "hx-denied-partial" {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
		if audit.denied != 1 {
			t.Errorf("denied audit = %d, want 1", audit.denied)
		}
	})

	t.Run("allowed passes through", func(t *testing.T) {
		h, _ := build(model.RoleSyndic)
		for _, hx := range []bool{false, true} {
			req := authenticatedRequest(true)
			if hx {
				req.Header.Set("HX-Request", "true")
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("hx=%v status=%d", hx, rec.Code)
			}
		}
	})
}
