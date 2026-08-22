package repositories

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	sharedstore "github.com/leoarkiteto/zelo/internal/shared/store"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/testutil"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	url := testutil.TestDatabaseURL("directory")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()
	db, err := sharedstore.Open(url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := sharedstore.Migrate(ctx, db, "../../../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		TRUNCATE service_provider_listings, service_categories, unit_occupancies,
		user_roles, units, condominiums, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return db
}

// seedDirectoryFixture creates a condominium, unit, user, and category, and
// returns their ids.
func seedDirectoryFixture(t *testing.T, db *sql.DB, categoryName string) (condoID, unitID, userID, categoryID string) {
	t.Helper()
	ctx := context.Background()
	if err := db.QueryRowContext(ctx,
		`INSERT INTO condominiums (name) VALUES ('Dir Condo') RETURNING id`).Scan(&condoID); err != nil {
		t.Fatalf("seed condo: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`INSERT INTO units (condominium_id, code) VALUES ($1, 'A-2') RETURNING id`, condoID).Scan(&unitID); err != nil {
		t.Fatalf("seed unit: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES ('dir@example.com', 'x') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO unit_occupancies (user_id, unit_id, occupancy_type) VALUES ($1, $2, 'owner')`,
		userID, unitID); err != nil {
		t.Fatalf("seed occupancy: %v", err)
	}
	cats := NewCategoryStore(db)
	id, err := cats.CreateCategory(ctx, model.ServiceCategory{CondominiumID: condoID, Name: categoryName})
	if err != nil {
		t.Fatalf("seed category: %v", err)
	}
	return condoID, unitID, userID, id
}

func TestCategoryStoreCreateAndRename(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	condoID, _, _, _ := seedDirectoryFixture(t, db, "Plumber")

	store := NewCategoryStore(db)
	cats, err := store.ListActiveCategories(ctx, condoID)
	if err != nil || len(cats) != 1 || cats[0].Name != "Plumber" {
		t.Fatalf("ListActiveCategories() = %v, %v", cats, err)
	}

	// Rename updates the name; a duplicate active name is rejected.
	if err := store.RenameCategory(ctx, cats[0].ID, "Pipe Repair"); err != nil {
		t.Fatalf("RenameCategory() = %v", err)
	}
	if _, err := store.CreateCategory(ctx, model.ServiceCategory{CondominiumID: condoID, Name: "Pipe Repair"}); !errors.Is(err, sharedstore.ErrDuplicate) {
		t.Fatalf("CreateCategory() duplicate = %v, want ErrDuplicate", err)
	}
}

func TestCategoryStoreDeactivate(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	condoID, _, _, _ := seedDirectoryFixture(t, db, "Electrician")

	store := NewCategoryStore(db)
	cats, _ := store.ListActiveCategories(ctx, condoID)
	if err := store.DeactivateCategory(ctx, cats[0].ID); err != nil {
		t.Fatalf("DeactivateCategory() = %v", err)
	}
	active, _ := store.ListActiveCategories(ctx, condoID)
	if len(active) != 0 {
		t.Fatalf("active categories after deactivate = %v, want none", active)
	}
	all, _ := store.ListCategories(ctx, condoID)
	if len(all) != 1 || all[0].Active {
		t.Fatalf("ListCategories() = %v, want one inactive category", all)
	}
}

func TestListingStoreLifecycle(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	condoID, unitID, userID, categoryID := seedDirectoryFixture(t, db, "Plumber")

	store := NewListingStore(db)
	listing := model.ServiceProviderListing{
		CondominiumID:         condoID,
		CategoryID:            categoryID,
		Name:                  "Ana's Plumbing",
		Phone:                 "+55 11 91234-5678",
		PhoneDigits:           "5511912345678",
		Notes:                 "Fast and reliable",
		RecommendedByUnitID:   unitID,
		RecommendedByUnitCode: "A-2",
		SubmittedBy:           userID,
	}
	id, err := store.CreateListing(ctx, listing)
	if err != nil {
		t.Fatalf("CreateListing() = %v", err)
	}
	got, err := store.GetListingByID(ctx, id)
	if err != nil {
		t.Fatalf("GetListingByID() = %v", err)
	}
	if got.Name != "Ana's Plumbing" || got.CategoryName != "Plumber" || got.RecommendedByUnitCode != "A-2" {
		t.Fatalf("GetListingByID() = %+v", got)
	}

	// Keyword search hits the name; category filter narrows.
	results, err := store.ListListings(ctx, condoID, "plumb", "")
	if err != nil || len(results) != 1 {
		t.Fatalf("ListListings('plumb') = %v, %v", results, err)
	}
	results, err = store.ListListings(ctx, condoID, "", categoryID)
	if err != nil || len(results) != 1 {
		t.Fatalf("ListListings(category) = %v, %v", results, err)
	}

	// Duplicate detection.
	dup, err := store.FindDuplicatePhone(ctx, condoID, "5511912345678", "")
	if err != nil || !dup {
		t.Fatalf("FindDuplicatePhone() = %v, %v, want true", dup, err)
	}
	dup, err = store.FindDuplicatePhone(ctx, condoID, "5511912345678", id)
	if err != nil || dup {
		t.Fatalf("FindDuplicatePhone(exclude) = %v, %v, want false", dup, err)
	}

	// Update.
	listing.ID = id
	listing.Name = "Ana's Pipes"
	if err := store.UpdateListing(ctx, listing); err != nil {
		t.Fatalf("UpdateListing() = %v", err)
	}

	// Delete; further reads fail.
	if err := store.DeleteListing(ctx, id, condoID); err != nil {
		t.Fatalf("DeleteListing() = %v", err)
	}
	if _, err := store.GetListingByID(ctx, id); !errors.Is(err, sharedstore.ErrNotFound) {
		t.Fatalf("GetListingByID() after delete = %v, want ErrNotFound", err)
	}
}

func TestUnitStoreGetActiveUnitForUser(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	condoID, unitID, userID, _ := seedDirectoryFixture(t, db, "Plumber")

	units := sharedstore.NewUnitStore(db)
	u, err := units.GetActiveUnitForUser(ctx, userID, condoID)
	if err != nil {
		t.Fatalf("GetActiveUnitForUser() = %v", err)
	}
	if u.ID != unitID || u.Code != "A-2" {
		t.Fatalf("GetActiveUnitForUser() = %+v", u)
	}
	_, err = units.GetActiveUnitForUser(ctx, "00000000-0000-0000-0000-000000000000", condoID)
	if !errors.Is(err, sharedstore.ErrNotFound) {
		t.Fatalf("GetActiveUnitForUser(unknown) = %v, want ErrNotFound", err)
	}
}
