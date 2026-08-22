package model

import "time"

// ServiceCategory is a speciality/category for directory listings, scoped to a
// condominium and maintained by the syndic (spec FR-011).
type ServiceCategory struct {
	ID            string
	CondominiumID string
	Name          string
	Active        bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ServiceProviderListing is a recommended service professional submitted by a
// resident (spec US1).
type ServiceProviderListing struct {
	ID                    string
	CondominiumID         string
	CategoryID            string
	CategoryName          string // display name joined from the category
	Name                  string
	Phone                 string
	PhoneDigits           string
	Notes                 string
	RecommendedByUnitID   string // empty when the unit row has been deleted
	RecommendedByUnitCode string
	SubmittedBy           string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
