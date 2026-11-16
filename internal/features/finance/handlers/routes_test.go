package handlers

import (
	"net/http"
	"testing"
)

// TestRegisterRoutesFullMux registers every finance route on a fresh ServeMux:
// a pattern conflict panics here instead of at startup.
func TestRegisterRoutesFullMux(t *testing.T) {
	deps := baseDeps(&fakeFinance{})
	deps.Quotas = &fakeQuotas{}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RegisterRoutes panicked: %v", r)
		}
	}()
	RegisterRoutes(http.NewServeMux(), deps)
}
