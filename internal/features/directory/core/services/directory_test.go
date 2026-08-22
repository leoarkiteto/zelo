package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// fakeAudit records audit events in memory.
type fakeAudit struct {
	events []model.AuditEvent
}

func (f *fakeAudit) RecordEvent(_ context.Context, userID *string, eventType model.AuditEventType, details map[string]any) error {
	f.events = append(f.events, model.AuditEvent{UserID: userID, EventType: eventType, Details: details})
	return nil
}

// fakeCategoryStore implements CategoryStore in memory.
type fakeCategoryStore struct {
	categories map[string]model.ServiceCategory
	seq        int
}

func newFakeCategoryStore() *fakeCategoryStore {
	return &fakeCategoryStore{categories: map[string]model.ServiceCategory{}}
}

func (f *fakeCategoryStore) CreateCategory(_ context.Context, c model.ServiceCategory) (string, error) {
	for _, existing := range f.categories {
		if existing.CondominiumID == c.CondominiumID && strings.EqualFold(existing.Name, c.Name) && existing.Active {
			return "", model.ErrDuplicate
		}
	}
	f.seq++
	id := "category-" + string(rune('a'+f.seq-1))
	c.ID = id
	c.Active = true
	f.categories[id] = c
	return id, nil
}

func (f *fakeCategoryStore) RenameCategory(_ context.Context, id, name string) error {
	c, ok := f.categories[id]
	if !ok {
		return model.ErrNotFound
	}
	for _, existing := range f.categories {
		if existing.ID != id && existing.CondominiumID == c.CondominiumID && strings.EqualFold(existing.Name, name) && existing.Active {
			return model.ErrDuplicate
		}
	}
	c.Name = name
	f.categories[id] = c
	return nil
}

func (f *fakeCategoryStore) DeactivateCategory(_ context.Context, id string) error {
	c, ok := f.categories[id]
	if !ok {
		return model.ErrNotFound
	}
	c.Active = false
	f.categories[id] = c
	return nil
}

func (f *fakeCategoryStore) GetCategoryByID(_ context.Context, id string) (model.ServiceCategory, error) {
	c, ok := f.categories[id]
	if !ok {
		return model.ServiceCategory{}, model.ErrNotFound
	}
	return c, nil
}

func (f *fakeCategoryStore) ListActiveCategories(_ context.Context, condominiumID string) ([]model.ServiceCategory, error) {
	var out []model.ServiceCategory
	for _, c := range f.categories {
		if c.CondominiumID == condominiumID && c.Active {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeCategoryStore) ListCategories(_ context.Context, condominiumID string) ([]model.ServiceCategory, error) {
	var out []model.ServiceCategory
	for _, c := range f.categories {
		if c.CondominiumID == condominiumID {
			out = append(out, c)
		}
	}
	return out, nil
}

// fakeListingStore implements ListingStore in memory.
type fakeListingStore struct {
	listings map[string]model.ServiceProviderListing
	seq      int
}

func newFakeListingStore() *fakeListingStore {
	return &fakeListingStore{listings: map[string]model.ServiceProviderListing{}}
}

func (f *fakeListingStore) CreateListing(_ context.Context, l model.ServiceProviderListing) (string, error) {
	f.seq++
	id := "listing-" + string(rune('a'+f.seq-1))
	l.ID = id
	f.listings[id] = l
	return id, nil
}

func (f *fakeListingStore) UpdateListing(_ context.Context, l model.ServiceProviderListing) error {
	existing, ok := f.listings[l.ID]
	if !ok {
		return model.ErrNotFound
	}
	l.RecommendedByUnitID = existing.RecommendedByUnitID
	l.RecommendedByUnitCode = existing.RecommendedByUnitCode
	l.SubmittedBy = existing.SubmittedBy
	f.listings[l.ID] = l
	return nil
}

func (f *fakeListingStore) DeleteListing(_ context.Context, id, condominiumID string) error {
	l, ok := f.listings[id]
	if !ok || l.CondominiumID != condominiumID {
		return model.ErrNotFound
	}
	delete(f.listings, id)
	return nil
}

func (f *fakeListingStore) GetListingByID(_ context.Context, id string) (model.ServiceProviderListing, error) {
	l, ok := f.listings[id]
	if !ok {
		return model.ServiceProviderListing{}, model.ErrNotFound
	}
	return l, nil
}

func (f *fakeListingStore) ListListings(_ context.Context, condominiumID, query, categoryID string) ([]model.ServiceProviderListing, error) {
	var out []model.ServiceProviderListing
	for _, l := range f.listings {
		if l.CondominiumID != condominiumID {
			continue
		}
		if categoryID != "" && l.CategoryID != categoryID {
			continue
		}
		if query != "" {
			hay := strings.ToLower(l.Name + " " + l.CategoryName + " " + l.Notes)
			if !strings.Contains(hay, strings.ToLower(query)) {
				continue
			}
		}
		out = append(out, l)
	}
	return out, nil
}

func (f *fakeListingStore) FindDuplicatePhone(_ context.Context, condominiumID, phoneDigits, excludeID string) (bool, error) {
	for _, l := range f.listings {
		if l.CondominiumID == condominiumID && l.PhoneDigits == phoneDigits && l.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

// fakeUnitResolver implements UnitResolver.
type fakeUnitResolver struct {
	unit model.Unit
	err  error
}

func (f *fakeUnitResolver) GetActiveUnitForUser(_ context.Context, _, _ string) (model.Unit, error) {
	if f.err != nil {
		return model.Unit{}, f.err
	}
	return f.unit, nil
}

func directoryFixture() (*DirectoryService, *fakeListingStore, *fakeCategoryStore, *fakeAudit, *fakeUnitResolver) {
	listings := newFakeListingStore()
	categories := newFakeCategoryStore()
	audit := &fakeAudit{}
	units := &fakeUnitResolver{unit: model.Unit{ID: "unit-1", CondominiumID: "condo-1", Code: "A-101"}}
	svc := &DirectoryService{Listings: listings, Categories: categories, Units: units, Audit: audit}
	return svc, listings, categories, audit, units
}

func seedCategory(t *testing.T, categories *fakeCategoryStore, id, condoID, name string, active bool) {
	t.Helper()
	categories.categories[id] = model.ServiceCategory{
		ID: id, CondominiumID: condoID, Name: name, Active: active,
	}
}

func TestCreateListingValid(t *testing.T) {
	svc, listings, categories, audit, _ := directoryFixture()
	seedCategory(t, categories, "cat-1", "condo-1", "Plumber", true)

	listing, duplicate, err := svc.CreateListing(context.Background(), "resident-1", "condo-1", ListingInput{
		Name: "Ana's Plumbing", CategoryID: "cat-1", Phone: "+55 11 91234-5678", Notes: "Fast",
	})
	if err != nil {
		t.Fatalf("CreateListing() error = %v", err)
	}
	if duplicate {
		t.Fatal("duplicate = true, want false")
	}
	if listing.RecommendedByUnitCode != "A-101" || listing.RecommendedByUnitID != "unit-1" {
		t.Fatalf("unit capture = %+v", listing)
	}
	if listing.SubmittedBy != "resident-1" {
		t.Fatalf("submitted_by = %q", listing.SubmittedBy)
	}
	if listing.PhoneDigits != "5511912345678" {
		t.Fatalf("phone digits = %q", listing.PhoneDigits)
	}
	if len(listings.listings) != 1 {
		t.Fatalf("listings = %d, want 1", len(listings.listings))
	}
	found := false
	for _, e := range audit.events {
		if e.EventType == model.AuditListingCreated {
			found = true
		}
	}
	if !found {
		t.Fatal("listing_created audit event not recorded")
	}
}

func TestCreateListingValidation(t *testing.T) {
	svc, _, categories, _, _ := directoryFixture()
	seedCategory(t, categories, "cat-1", "condo-1", "Plumber", true)
	seedCategory(t, categories, "cat-2", "condo-1", "Retired", false)

	cases := []struct {
		name  string
		input ListingInput
		want  error
	}{
		{"empty name", ListingInput{Name: "  ", CategoryID: "cat-1", Phone: "+5511999999999"}, ErrInvalidListing},
		{"bad phone chars", ListingInput{Name: "X", CategoryID: "cat-1", Phone: "abc"}, ErrInvalidPhone},
		{"short phone", ListingInput{Name: "X", CategoryID: "cat-1", Phone: "123456"}, ErrInvalidPhone},
		{"unknown category", ListingInput{Name: "X", CategoryID: "missing", Phone: "+5511999999999"}, ErrInvalidCategory},
		{"inactive category", ListingInput{Name: "X", CategoryID: "cat-2", Phone: "+5511999999999"}, ErrInvalidCategory},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := svc.CreateListing(context.Background(), "resident-1", "condo-1", c.input)
			if !errors.Is(err, c.want) {
				t.Fatalf("CreateListing() error = %v, want %v", err, c.want)
			}
		})
	}
}

func TestCreateListingNoUnit(t *testing.T) {
	svc, _, categories, _, units := directoryFixture()
	seedCategory(t, categories, "cat-1", "condo-1", "Plumber", true)
	units.err = model.ErrNotFound

	_, _, err := svc.CreateListing(context.Background(), "resident-1", "condo-1", ListingInput{
		Name: "Ana", CategoryID: "cat-1", Phone: "+5511999999999",
	})
	if !errors.Is(err, ErrNoUnit) {
		t.Fatalf("CreateListing() error = %v, want ErrNoUnit", err)
	}
}

func TestCreateListingDuplicateFlow(t *testing.T) {
	svc, listings, categories, _, _ := directoryFixture()
	seedCategory(t, categories, "cat-1", "condo-1", "Plumber", true)
	in := ListingInput{Name: "Ana's Plumbing", CategoryID: "cat-1", Phone: "+55 11 91234-5678"}
	if _, _, err := svc.CreateListing(context.Background(), "resident-1", "condo-1", in); err != nil {
		t.Fatalf("first CreateListing() = %v", err)
	}

	// Same phone, no confirmation → duplicate warning, not saved.
	in2 := ListingInput{Name: "Ana's Plumbing (2)", CategoryID: "cat-1", Phone: "55 11 91234 5678"}
	_, duplicate, err := svc.CreateListing(context.Background(), "resident-2", "condo-1", in2)
	if err != nil || !duplicate {
		t.Fatalf("duplicate CreateListing() = dup:%v err:%v, want dup:true err:nil", duplicate, err)
	}
	if len(listings.listings) != 1 {
		t.Fatalf("listings = %d, want 1 (duplicate not saved)", len(listings.listings))
	}

	// With confirmation → saved.
	in2.ConfirmDuplicate = true
	_, duplicate, err = svc.CreateListing(context.Background(), "resident-2", "condo-1", in2)
	if err != nil || duplicate {
		t.Fatalf("confirmed CreateListing() = dup:%v err:%v", duplicate, err)
	}
	if len(listings.listings) != 2 {
		t.Fatalf("listings = %d, want 2", len(listings.listings))
	}
}

func TestSearchListings(t *testing.T) {
	svc, _, categories, _, _ := directoryFixture()
	seedCategory(t, categories, "cat-1", "condo-1", "Plumber", true)
	_, _, err := svc.CreateListing(context.Background(), "resident-1", "condo-1", ListingInput{
		Name: "Ana's Plumbing", CategoryID: "cat-1", Phone: "+5511999999999",
	})
	if err != nil {
		t.Fatalf("seed Ana's Plumbing: %v", err)
	}
	_, _, err = svc.CreateListing(context.Background(), "resident-1", "condo-1", ListingInput{
		Name: "Bob Electrician", CategoryID: "cat-1", Phone: "+5511888887777",
	})
	if err != nil {
		t.Fatalf("seed Bob Electrician: %v", err)
	}

	results, err := svc.SearchListings(context.Background(), "condo-1", "electrician", "")
	if err != nil || len(results) != 1 || results[0].Name != "Bob Electrician" {
		t.Fatalf("SearchListings() = %v, %v", results, err)
	}
	results, err = svc.SearchListings(context.Background(), "condo-1", "zzz-no-match", "")
	if err != nil || len(results) != 0 {
		t.Fatalf("SearchListings(no match) = %v, %v", results, err)
	}
	if _, err := svc.SearchListings(context.Background(), "condo-1", "", "missing-category"); !errors.Is(err, ErrInvalidCategory) {
		t.Fatalf("SearchListings(bad category) = %v, want ErrInvalidCategory", err)
	}
}

func TestEditAndDeleteListing(t *testing.T) {
	svc, listings, categories, audit, _ := directoryFixture()
	seedCategory(t, categories, "cat-1", "condo-1", "Plumber", true)
	in := ListingInput{Name: "Ana's Plumbing", CategoryID: "cat-1", Phone: "+5511999999999"}
	created, _, err := svc.CreateListing(context.Background(), "resident-1", "condo-1", in)
	if err != nil {
		t.Fatalf("CreateListing() = %v", err)
	}

	if err := svc.EditListing(context.Background(), "syndic-1", created.ID, "condo-1", ListingInput{
		Name: "Ana's Pipes", CategoryID: "cat-1", Phone: "+5511988887777",
	}); err != nil {
		t.Fatalf("EditListing() = %v", err)
	}
	got, _ := listings.GetListingByID(context.Background(), created.ID)
	if got.Name != "Ana's Pipes" || got.PhoneDigits != "5511988887777" {
		t.Fatalf("listing after edit = %+v", got)
	}
	if err := svc.EditListing(context.Background(), "syndic-1", "no-such", "condo-1", in); !errors.Is(err, ErrNotFound) {
		t.Fatalf("EditListing(unknown) = %v, want ErrNotFound", err)
	}

	if err := svc.DeleteListing(context.Background(), "syndic-1", created.ID, "condo-1"); err != nil {
		t.Fatalf("DeleteListing() = %v", err)
	}
	if len(listings.listings) != 0 {
		t.Fatalf("listings after delete = %d, want 0", len(listings.listings))
	}
	events := 0
	for _, e := range audit.events {
		if e.EventType == model.AuditListingEdited || e.EventType == model.AuditListingDeleted {
			events++
		}
	}
	if events != 2 {
		t.Fatalf("moderation audit events = %d, want 2", events)
	}
}

func TestCategoryManagement(t *testing.T) {
	svc, _, categories, audit, _ := directoryFixture()
	seedCategory(t, categories, "cat-1", "condo-1", "Plumber", true)

	id, err := svc.CreateCategory(context.Background(), "syndic-1", "condo-1", "Carpenter")
	if err != nil {
		t.Fatalf("CreateCategory() = %v", err)
	}
	if _, err := svc.CreateCategory(context.Background(), "syndic-1", "condo-1", "plumber"); !errors.Is(err, model.ErrDuplicate) {
		t.Fatalf("CreateCategory(duplicate) = %v, want ErrDuplicate", err)
	}
	if err := svc.RenameCategory(context.Background(), "syndic-1", id, "condo-1", "Carpentry"); err != nil {
		t.Fatalf("RenameCategory() = %v", err)
	}
	if err := svc.DeactivateCategory(context.Background(), "syndic-1", id, "condo-1"); err != nil {
		t.Fatalf("DeactivateCategory() = %v", err)
	}
	c, _ := categories.GetCategoryByID(context.Background(), id)
	if c.Active {
		t.Fatal("category still active after deactivate")
	}

	// Scoping: a category from another condominium is not manageable.
	if err := svc.RenameCategory(context.Background(), "syndic-1", "cat-1", "condo-2", "Other"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RenameCategory(cross-condo) = %v, want ErrNotFound", err)
	}
	if err := svc.DeactivateCategory(context.Background(), "syndic-1", "cat-1", "condo-2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeactivateCategory(cross-condo) = %v, want ErrNotFound", err)
	}
	events := 0
	for _, e := range audit.events {
		switch e.EventType {
		case model.AuditCategoryCreated, model.AuditCategoryRenamed, model.AuditCategoryDeactivated:
			events++
		}
	}
	if events != 3 {
		t.Fatalf("category audit events = %d, want 3", events)
	}
}
