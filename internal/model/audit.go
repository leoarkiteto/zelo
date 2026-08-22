package model

import "time"

// AuditEventType enumerates security-relevant events (spec FR-013).
type AuditEventType string

const (
	AuditSignIn        AuditEventType = "sign_in"
	AuditFailedSignIn  AuditEventType = "failed_sign_in"
	AuditSignOut       AuditEventType = "sign_out"
	AuditAccountLocked AuditEventType = "account_locked"
	AuditRoleGranted   AuditEventType = "role_granted"
	AuditRoleRevoked   AuditEventType = "role_revoked"
	AuditAccessDenied  AuditEventType = "access_denied"
)

// Valid reports whether t is a supported audit event type.
func (t AuditEventType) Valid() bool {
	switch t {
	case AuditSignIn, AuditFailedSignIn, AuditSignOut, AuditAccountLocked,
		AuditRoleGranted, AuditRoleRevoked, AuditAccessDenied:
		return true
	default:
		return false
	}
}

// AuditEvent is a security-relevant event record.
type AuditEvent struct {
	ID        string
	UserID    *string
	EventType AuditEventType
	Details   map[string]any
	CreatedAt time.Time
}
