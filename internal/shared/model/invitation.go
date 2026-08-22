package model

import "time"

// InvitationStatus is the lifecycle status of a registration invitation.
type InvitationStatus string

const (
	InvitationPending  InvitationStatus = "pending"
	InvitationAccepted InvitationStatus = "accepted"
	InvitationExpired  InvitationStatus = "expired"
	InvitationRevoked  InvitationStatus = "revoked"
)

// Invitation is a syndic-issued link/code for registration.
type Invitation struct {
	ID            string
	TokenHash     string
	CondominiumID string
	UnitID        string
	InvitedRole   Role
	InvitedEmail  string
	Status        InvitationStatus
	ExpiresAt     time.Time
	CreatedBy     string
	CreatedAt     time.Time
	AcceptedAt    *time.Time
}
