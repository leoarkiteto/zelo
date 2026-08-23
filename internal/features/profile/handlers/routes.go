package handlers

import (
	"net/http"

	"github.com/leoarkiteto/zelo/internal/shared/middleware"
)

// RegisterRoutes registers the profile routes on mux. Both routes require an
// authenticated session; anonymous requests are redirected to /login.
func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	h := &Handler{deps: deps}

	mux.Handle("GET /profile", middleware.RequireAuth(http.HandlerFunc(h.profileGET)))
	mux.Handle("POST /profile/language", middleware.RequireAuth(http.HandlerFunc(h.profileLanguagePOST)))
}
