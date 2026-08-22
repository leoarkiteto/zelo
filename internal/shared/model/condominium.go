package model

import "time"

// OccupancyType describes the relationship between a user and a unit.
type OccupancyType string

const (
	OccupancyOwner  OccupancyType = "owner"
	OccupancyTenant OccupancyType = "tenant"
)

// Condominium is the building and community being managed.
type Condominium struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// Unit is a property within a condominium.
type Unit struct {
	ID            string
	CondominiumID string
	Code          string
}

// UnitOccupancy ties a user to a unit as owner or tenant.
type UnitOccupancy struct {
	ID            string
	UserID        string
	UnitID        string
	OccupancyType OccupancyType
	StartedAt     time.Time
	EndedAt       *time.Time
}
