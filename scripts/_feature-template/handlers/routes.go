// Package handlers contains {{FEATURE}} HTTP handlers and route registration.
package handlers

import "net/http"

// Deps are the collaborators used by {{FEATURE}} handlers.
type Deps struct{}

// Handler bundles dependencies for {{FEATURE}} handler methods.
type Handler struct {
	deps Deps
}

// RegisterRoutes registers the {{FEATURE}} routes on mux.
func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	_ = &Handler{deps: deps}
	// Register your feature's routes here.
}
