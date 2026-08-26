package model

import "time"

// AuditEventType enumerates security-relevant events (spec FR-013).
type AuditEventType string

const (
	AuditSignIn              AuditEventType = "sign_in"
	AuditFailedSignIn        AuditEventType = "failed_sign_in"
	AuditSignOut             AuditEventType = "sign_out"
	AuditAccountLocked       AuditEventType = "account_locked"
	AuditRoleGranted         AuditEventType = "role_granted"
	AuditRoleRevoked         AuditEventType = "role_revoked"
	AuditAccessDenied        AuditEventType = "access_denied"
	AuditListingCreated      AuditEventType = "listing_created"
	AuditListingEdited       AuditEventType = "listing_edited"
	AuditListingDeleted      AuditEventType = "listing_deleted"
	AuditCategoryCreated     AuditEventType = "category_created"
	AuditCategoryRenamed     AuditEventType = "category_renamed"
	AuditCategoryDeactivated AuditEventType = "category_deactivated"
	AuditAccountCreated      AuditEventType = "account_created"
	AuditAccountEdited       AuditEventType = "account_edited"
	AuditAccountSettled      AuditEventType = "account_settled"
	AuditAccountCanceled     AuditEventType = "account_canceled"
	AuditTicketCreated       AuditEventType = "ticket_created"
	AuditTicketReplied       AuditEventType = "ticket_replied"
	AuditTicketClosed        AuditEventType = "ticket_closed"
)

// Valid reports whether t is a supported audit event type.
func (t AuditEventType) Valid() bool {
	switch t {
	case AuditSignIn, AuditFailedSignIn, AuditSignOut, AuditAccountLocked,
		AuditRoleGranted, AuditRoleRevoked, AuditAccessDenied,
		AuditListingCreated, AuditListingEdited, AuditListingDeleted,
		AuditCategoryCreated, AuditCategoryRenamed, AuditCategoryDeactivated,
		AuditAccountCreated, AuditAccountEdited, AuditAccountSettled, AuditAccountCanceled,
		AuditTicketCreated, AuditTicketReplied, AuditTicketClosed:
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
