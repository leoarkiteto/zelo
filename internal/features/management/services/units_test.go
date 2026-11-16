package services

import (
	"context"
	"errors"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/permille"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

type fakeUnitStore struct {
	units     []model.Unit
	createErr error
	updateErr error
	creates   int
	updates   int
}

func (f *fakeUnitStore) ListUnitsForCondominium(context.Context, string) ([]model.Unit, error) {
	return f.units, nil
}

func (f *fakeUnitStore) CreateUnit(_ context.Context, condo, code string, ppm int) (model.Unit, error) {
	if f.createErr != nil {
		return model.Unit{}, f.createErr
	}
	f.creates++
	u := model.Unit{ID: "new", CondominiumID: condo, Code: code, SharePPM: ppm}
	f.units = append(f.units, u)
	return u, nil
}

func (f *fakeUnitStore) UpdateUnit(_ context.Context, _, id, code string, ppm int) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updates++
	for i := range f.units {
		if f.units[i].ID == id {
			f.units[i].Code, f.units[i].SharePPM = code, ppm
		}
	}
	return nil
}

func TestListSumAndDiff(t *testing.T) {
	svc := &UnitService{Units: &fakeUnitStore{units: []model.Unit{{ID: "1", Code: "A", SharePPM: 400000}, {ID: "2", Code: "B", SharePPM: 350000}}}}
	l, err := svc.List(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if l.SumPPM != 750000 || l.DiffPPM != 250000 {
		t.Fatalf("sum=%d diff=%d", l.SumPPM, l.DiffPPM)
	}
}

func TestCreateAuditsAndRejects(t *testing.T) {
	st := &fakeUnitStore{}
	au := &fakeAudit{}
	svc := &UnitService{Units: st, Audit: au}
	if _, err := svc.Create(context.Background(), "u1", "c1", " C ", "250"); err != nil {
		t.Fatal(err)
	}
	if st.units[0].Code != "C" || st.units[0].SharePPM != 250000 {
		t.Fatalf("stored %+v", st.units[0])
	}
	if len(au.events) != 1 || au.events[0].EventType != model.AuditUnitCreated {
		t.Fatalf("events = %+v", au.events)
	}
	for _, share := range []string{"-1", "1000.5", "1.2345"} {
		if _, err := svc.Create(context.Background(), "u1", "c1", "D", share); !errors.Is(err, permille.ErrInvalid) {
			t.Errorf("share %q err = %v", share, err)
		}
	}
	if _, err := svc.Create(context.Background(), "u1", "c1", " ", "1"); !errors.Is(err, ErrCodeRequired) {
		t.Errorf("empty code err = %v", err)
	}
	if st.creates != 1 || len(au.events) != 1 {
		t.Fatalf("rejected input must not write or audit")
	}
	st.createErr = store.ErrConflict
	if _, err := svc.Create(context.Background(), "u1", "c1", "A", "1"); !errors.Is(err, ErrDuplicateCode) {
		t.Errorf("duplicate err = %v", err)
	}
	if len(au.events) != 1 {
		t.Fatalf("duplicate must not audit")
	}
}

func TestUpdateAuditsAndNotFound(t *testing.T) {
	st := &fakeUnitStore{units: []model.Unit{{ID: "1", Code: "A", SharePPM: 100000}}}
	au := &fakeAudit{}
	svc := &UnitService{Units: st, Audit: au}
	if err := svc.Update(context.Background(), "u1", "c1", "1", "A", "400"); err != nil {
		t.Fatal(err)
	}
	if st.units[0].SharePPM != 400000 || len(au.events) != 1 || au.events[0].EventType != model.AuditUnitUpdated {
		t.Fatalf("units=%+v events=%+v", st.units, au.events)
	}
	if err := svc.Update(context.Background(), "u1", "c1", "zz", "A", "1"); !errors.Is(err, ErrUnitNotFound) {
		t.Errorf("missing err = %v", err)
	}
	st.updateErr = store.ErrConflict
	if err := svc.Update(context.Background(), "u1", "c1", "1", "B", "1"); !errors.Is(err, ErrDuplicateCode) {
		t.Errorf("dup err = %v", err)
	}
}
