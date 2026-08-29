package store

import (
	"context"
	"database/sql"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/testutil"
)

func openStoreTestDB(t *testing.T) *sql.DB {
	t.Helper()
	url := testutil.TestDatabaseURL("store")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()
	db, err := Open(url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrate(ctx, db, "../../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		TRUNCATE audit_events, invitations, service_provider_listings,
		service_categories, unit_occupancies, user_roles, units, condominiums,
		users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return db
}

func createUserForLanguageTest(t *testing.T, db *sql.DB) string {
	t.Helper()
	ctx := context.Background()
	us := NewUserStore(db)
	id, err := us.CreateUser(ctx, model.User{
		Email:        "lang@example.com",
		PasswordHash: "hash",
		Status:       model.UserStatusActive,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

func TestUserStoreLanguagePreferenceRoundTrip(t *testing.T) {
	db := openStoreTestDB(t)
	ctx := context.Background()
	us := NewUserStore(db)
	id := createUserForLanguageTest(t, db)

	// Unset preference reads as empty (default English).
	u, err := us.GetUserByID(ctx, id)
	if err != nil {
		t.Fatalf("get user by id: %v", err)
	}
	if u.LanguagePreference != "" {
		t.Fatalf("initial preference = %q, want empty", u.LanguagePreference)
	}

	// Update persists.
	if err := us.UpdateLanguagePreference(ctx, id, "pt-br"); err != nil {
		t.Fatalf("update language preference: %v", err)
	}

	// Both readers see the saved preference.
	u, err = us.GetUserByID(ctx, id)
	if err != nil {
		t.Fatalf("get user by id after update: %v", err)
	}
	if u.LanguagePreference != "pt-br" {
		t.Fatalf("preference by id = %q, want %q", u.LanguagePreference, "pt-br")
	}
	byEmail, err := us.GetUserByEmail(ctx, "lang@example.com")
	if err != nil {
		t.Fatalf("get user by email: %v", err)
	}
	if byEmail.LanguagePreference != "pt-br" {
		t.Fatalf("preference by email = %q, want %q", byEmail.LanguagePreference, "pt-br")
	}
}

func TestUserStoreUpdateLanguagePreferenceUnknownUser(t *testing.T) {
	db := openStoreTestDB(t)
	ctx := context.Background()
	us := NewUserStore(db)
	if err := us.UpdateLanguagePreference(ctx, "00000000-0000-0000-0000-000000000000", "en"); err == nil {
		t.Fatal("update for unknown user = nil, want ErrNotFound")
	}
}
