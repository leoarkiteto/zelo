package model

import "time"

// Session is a server-side session record referenced by a cookie.
type Session struct {
	TokenHash     string
	UserID        string
	CondominiumID string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	CSRFToken     string
}
