package handlers

import (
	"net/http"

	"github.com/leoarkiteto/zelo/internal/shared/middleware"
)

// RegisterRoutes registers the authentication routes on mux.
func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	h := &Handler{deps: deps}

	mux.HandleFunc("GET /register", h.registerGET)
	mux.HandleFunc("POST /register", h.registerPOST)
	mux.HandleFunc("GET /login", h.loginGET)
	mux.HandleFunc("POST /login", h.loginPOST)
	mux.HandleFunc("GET /password/forgot", h.forgotGET)
	mux.HandleFunc("POST /password/forgot", h.forgotPOST)
	mux.HandleFunc("GET /password/reset", h.resetGET)
	mux.HandleFunc("POST /password/reset", h.resetPOST)
	mux.Handle("POST /logout", middleware.RequireAuth(http.HandlerFunc(h.logoutPOST)))
}
