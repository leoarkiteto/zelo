// Package model contains domain types shared across layers.
package model

import "time"

// Role is one of the three supported RBAC roles.
type Role string

const (
	RoleSyndic Role = "syndic"
	RoleOwner  Role = "owner"
	RoleTenant Role = "tenant"
)

// Valid reports whether r is a supported role.
func (r Role) Valid() bool {
	return r == RoleSyndic || r == RoleOwner || r == RoleTenant
}

// UserStatus is the lifecycle status of a user account.
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

// User is a person with an account.
type User struct {
	ID                string
	Email             string
	PasswordHash      string
	Status            UserStatus
	FailedSignInCount int
	LockedUntil       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// RoleAssignment is an RBAC assignment scoped to a condominium.
type RoleAssignment struct {
	UserID        string
	CondominiumID string
	Role          Role
	GrantedAt     time.Time
	RevokedAt     *time.Time
}
