package store

import (
	"context"
	"errors"
	"testing"
)

func TestUnitStoreCreateUpdateAndOccupancies(t *testing.T) {
	db := openStoreTestDB(t)
	ctx := context.Background()
	var condo, other, user string
	if err := db.QueryRowContext(ctx, `INSERT INTO condominiums (name) VALUES ('A') RETURNING id`).Scan(&condo); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO condominiums (name) VALUES ('B') RETURNING id`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	user = createUserForLanguageTest(t, db)
	us := NewUnitStore(db)

	b, err := us.CreateUnit(ctx, condo, "B", 250000)
	if err != nil || b.ID == "" || b.SharePPM != 250000 {
		t.Fatalf("create B = %+v, %v", b, err)
	}
	a, err := us.CreateUnit(ctx, condo, "A", 0)
	if err != nil {
		t.Fatalf("create A (share 0 allowed): %v", err)
	}
	if _, err := us.CreateUnit(ctx, condo, "A", 10); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate code = %v, want ErrConflict", err)
	}
	if _, err := us.CreateUnit(ctx, condo, "C", 1000001); err == nil {
		t.Fatal("share above 1000000 must be rejected")
	}

	if err := us.UpdateUnit(ctx, condo, a.ID, "A1", 5000); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := us.UpdateUnit(ctx, condo, a.ID, "B", 5000); !errors.Is(err, ErrConflict) {
		t.Fatalf("update duplicate = %v, want ErrConflict", err)
	}
	if err := us.UpdateUnit(ctx, other, a.ID, "X", 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update foreign condominium = %v, want ErrNotFound", err)
	}

	units, err := us.ListUnitsForCondominium(ctx, condo)
	if err != nil || len(units) != 2 || units[0].Code != "A1" || units[0].SharePPM != 5000 || units[1].SharePPM != 250000 {
		t.Fatalf("list = %+v, %v", units, err)
	}

	// Occupy B as owner and tenant (two rows, one unit) and A1 as tenant.
	for _, q := range []string{
		`INSERT INTO unit_occupancies (user_id, unit_id, occupancy_type, started_at) VALUES ($1, $2, 'owner', now() - interval '2 days')`,
		`INSERT INTO unit_occupancies (user_id, unit_id, occupancy_type, started_at) VALUES ($1, $2, 'tenant', now() - interval '1 day')`,
	} {
		if _, err := db.ExecContext(ctx, q, user, b.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO unit_occupancies (user_id, unit_id, occupancy_type) VALUES ($1, $2, 'tenant')`, user, a.ID); err != nil {
		t.Fatal(err)
	}

	occ, err := us.ListActiveOccupanciesForUser(ctx, user, condo)
	if err != nil || len(occ) != 2 {
		t.Fatalf("occupancies = %+v, %v", occ, err)
	}
	if occ[0].Unit.Code != "A1" || occ[1].Unit.Code != "B" || occ[1].OccupancyType != "owner" {
		t.Fatalf("order/earliest wrong: %+v", occ)
	}
	first, err := us.GetActiveUnitForUser(ctx, user, condo)
	if err != nil || first.Code != "A1" {
		t.Fatalf("GetActiveUnitForUser = %+v, %v; want A1", first, err)
	}
	if _, err := us.GetActiveUnitForUser(ctx, user, other); !errors.Is(err, ErrNotFound) {
		t.Fatalf("no occupancy = %v, want ErrNotFound", err)
	}
}
