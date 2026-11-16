package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/money"
)

func (f *fakeQuotaStore) CreateExtraordinary(_ context.Context, e NewExtraordinary, in []NewInstallment) (string, error) {
	id := f.id("x")
	f.extras = append(f.extras, ExtraordinarySummary{
		ID: id, CondominiumID: e.CondominiumID, Description: e.Description, TotalCents: e.TotalCents,
		Division: e.Division, Installments: e.Installments, FirstDueDate: e.FirstDueDate, Status: domain.IssuanceIssued,
	})
	for _, i := range in {
		for _, c := range i.Charges {
			f.extraCharges = append(f.extraCharges, fExtraCharge{NewCharge: c, extraID: id, number: i.Number, status: domain.StatusPending})
		}
	}
	return id, nil
}

func (f *fakeQuotaStore) ListExtraordinaries(context.Context, string) ([]ExtraordinarySummary, error) {
	return f.extras, nil
}

func (f *fakeQuotaStore) GetExtraordinary(_ context.Context, _, id string) (ExtraordinaryDetail, error) {
	for _, e := range f.extras {
		if e.ID == id {
			return ExtraordinaryDetail{ExtraordinarySummary: e}, nil
		}
	}
	return ExtraordinaryDetail{}, model.ErrNotFound
}

type fLockedExtraordinary struct {
	f   *fakeQuotaStore
	idx int
}

func (l fLockedExtraordinary) Extraordinary() ExtraordinarySummary { return l.f.extras[l.idx] }

func (l fLockedExtraordinary) CancelPendingCharges() (int, int, error) {
	total, affected := 0, 0
	for i, c := range l.f.extraCharges {
		if c.extraID != l.f.extras[l.idx].ID {
			continue
		}
		total++
		if c.status == domain.StatusPending {
			l.f.extraCharges[i].status = domain.StatusCanceled
			affected++
		}
	}
	return affected, total, nil
}

func (l fLockedExtraordinary) MarkCanceled(at time.Time) error {
	l.f.extras[l.idx].Status = domain.IssuanceCanceled
	l.f.extras[l.idx].CanceledAt = &at
	return nil
}

func (f *fakeQuotaStore) WithLockedExtraordinary(_ context.Context, _, id string, fn func(context.Context, LockedExtraordinary) error) error {
	for i, e := range f.extras {
		if e.ID == id {
			f.locks = append(f.locks, "extraordinary")
			return f.tx(func() error { return fn(context.Background(), fLockedExtraordinary{f, i}) })
		}
	}
	return model.ErrNotFound
}

func extraInput() ExtraordinaryInput {
	return ExtraordinaryInput{
		Description: "Roof repair", TotalCents: 100000, Division: DivisionShare, Installments: 3,
		FirstDueDate: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC),
	}
}

func extraSvc() (*QuotaService, *fakeQuotaStore, *recAudit) {
	svc, st, au := newQuotaSvc(money.EUR)
	approvedBudget(st) // 50/30/20 units
	return svc, st, au
}

func TestPreviewExtraordinaryByShare(t *testing.T) {
	svc, _, _ := extraSvc()
	pv, err := svc.PreviewExtraordinary(context.Background(), condo, extraInput())
	if err != nil {
		t.Fatal(err)
	}
	// 100000 / 3 -> 33334, 33333, 33333; each split 50/30/20.
	wantInst := []int64{33334, 33333, 33333}
	var sum int64
	for i, in := range pv.Installments {
		if in.TotalCents != wantInst[i] || in.Number != i+1 {
			t.Errorf("installment %d = %+v", i, in)
		}
		sum += in.TotalCents
	}
	if sum != 100000 {
		t.Fatalf("installments sum = %d", sum)
	}
	if got := pv.Installments[1].DueDate; !got.Equal(time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("due 2 = %v", got)
	}
	if got := pv.Installments[2].DueDate; !got.Equal(time.Date(2027, 1, 10, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("due 3 (next year) = %v", got)
	}
	var total int64
	for _, u := range pv.Units {
		var s int64
		for _, c := range u.PerInstallmentCents {
			s += c
		}
		if s != u.TotalCents {
			t.Errorf("unit %s total %d != %d", u.Code, u.TotalCents, s)
		}
		total += u.TotalCents
	}
	if total != 100000 {
		t.Fatalf("units sum = %d", total)
	}
	if pv.Units[0].PerInstallmentCents[0] != 16667 { // 33334 * 50%
		t.Errorf("A-1 inst 1 = %d", pv.Units[0].PerInstallmentCents[0])
	}
}

func TestPreviewExtraordinaryEqualAndRefusals(t *testing.T) {
	svc, st, _ := extraSvc()
	in := extraInput()
	in.Division, in.Installments = DivisionEqual, 1
	in.TotalCents = 100
	pv, err := svc.PreviewExtraordinary(context.Background(), condo, in)
	if err != nil {
		t.Fatal(err)
	}
	got := []int64{pv.Units[0].TotalCents, pv.Units[1].TotalCents, pv.Units[2].TotalCents}
	if got[0] != 34 || got[1] != 33 || got[2] != 33 { // extra cent to the lowest code
		t.Errorf("equal split = %v", got)
	}

	bad := func(name string, mut func(*ExtraordinaryInput), want error) {
		t.Helper()
		in := extraInput()
		mut(&in)
		if _, err := svc.PreviewExtraordinary(context.Background(), condo, in); !errors.Is(err, want) {
			t.Errorf("%s: err = %v, want %v", name, err, want)
		}
	}
	bad("empty description", func(i *ExtraordinaryInput) { i.Description = "  " }, domain.ErrInvalidExtraordinary)
	bad("long description", func(i *ExtraordinaryInput) { i.Description = string(make([]rune, 121)) + "x" }, domain.ErrInvalidExtraordinary)
	bad("zero total", func(i *ExtraordinaryInput) { i.TotalCents = 0 }, domain.ErrInvalidExtraordinary)
	bad("day 29", func(i *ExtraordinaryInput) { i.FirstDueDate = time.Date(2026, 1, 29, 0, 0, 0, 0, time.UTC) }, domain.ErrInvalidExtraordinary)
	bad("division", func(i *ExtraordinaryInput) { i.Division = "x" }, domain.ErrInvalidExtraordinary)
	bad("0 installments", func(i *ExtraordinaryInput) { i.Installments = 0 }, domain.ErrInvalidInstallments)
	bad("25 installments", func(i *ExtraordinaryInput) { i.Installments = 25 }, domain.ErrInvalidInstallments)

	st.units[2].SharePPM = 100000 // sums to 90 %
	_, err = svc.PreviewExtraordinary(context.Background(), condo, extraInput())
	var se SharesError
	if !errors.As(err, &se) || se.DiffPPM() != -100000 || !errors.Is(err, domain.ErrSharesNot100) {
		t.Fatalf("shares refusal = %v", err)
	}
	// Equal division does not need 100 %.
	if _, err := svc.PreviewExtraordinary(context.Background(), condo, in); err != nil {
		t.Fatalf("equal with 90%% = %v", err)
	}
	st.units = nil
	if _, err := svc.PreviewExtraordinary(context.Background(), condo, in); !errors.Is(err, domain.ErrNoChargeableUnits) {
		t.Fatalf("no units = %v", err)
	}
}

func TestCreateExtraordinary(t *testing.T) {
	svc, st, au := extraSvc()
	st.units = append(st.units, domain.ShareInput{UnitID: "u4", Code: "A-4", SharePPM: 0})
	st.units[2].SharePPM = 200000
	id, err := svc.CreateExtraordinary(context.Background(), "syn", condo, extraInput())
	if err != nil || id == "" {
		t.Fatalf("create = %q %v", id, err)
	}
	if len(st.extraCharges) != 9 { // 3 units x 3 installments, the zero-share unit gets none
		t.Fatalf("charges = %d", len(st.extraCharges))
	}
	for _, c := range st.extraCharges {
		if c.AmountCents <= 0 || c.ReserveCents != 0 || c.BaseCents != c.AmountCents || c.SharePPM == 0 || c.Title == "" || c.UnitID == "u4" {
			t.Errorf("bad charge %+v", c)
		}
	}
	if au.count(model.AuditExtraordinaryQuotaCreated) != 1 {
		t.Fatalf("audit = %v", au.types)
	}
	if len(st.issuances) != 0 {
		t.Fatalf("ordinary issuances touched: %d", len(st.issuances))
	}
	if _, err := svc.CreateExtraordinary(context.Background(), "syn", condo, ExtraordinaryInput{}); !errors.Is(err, domain.ErrInvalidExtraordinary) {
		t.Fatalf("invalid create = %v", err)
	}
	if len(st.extras) != 1 {
		t.Fatalf("extras after refusal = %d", len(st.extras))
	}
}

func TestCancelExtraordinary(t *testing.T) {
	svc, st, au := extraSvc()
	id, err := svc.CreateExtraordinary(context.Background(), "syn", condo, extraInput())
	if err != nil {
		t.Fatal(err)
	}
	st.extraCharges[0].status = domain.StatusSettled
	if err := svc.CancelExtraordinary(context.Background(), "syn", condo, id); !errors.Is(err, domain.ErrExtraordinaryHasSettled) {
		t.Fatalf("cancel with settled = %v", err)
	}
	for i, c := range st.extraCharges[1:] {
		if c.status != domain.StatusPending {
			t.Errorf("charge %d not rolled back: %s", i+1, c.status)
		}
	}
	if st.extras[0].Status != domain.IssuanceIssued || au.count(model.AuditExtraordinaryQuotaCanceled) != 0 {
		t.Fatal("state changed on refusal")
	}
	st.extraCharges[0].status = domain.StatusPending
	if err := svc.CancelExtraordinary(context.Background(), "syn", condo, id); err != nil {
		t.Fatal(err)
	}
	for _, c := range st.extraCharges {
		if c.status != domain.StatusCanceled {
			t.Fatalf("charge not canceled: %+v", c)
		}
	}
	if st.extras[0].Status != domain.IssuanceCanceled || au.count(model.AuditExtraordinaryQuotaCanceled) != 1 {
		t.Fatalf("status/audit = %s %v", st.extras[0].Status, au.types)
	}
	if err := svc.CancelExtraordinary(context.Background(), "syn", condo, id); !errors.Is(err, ErrIssuanceNotActive) {
		t.Fatalf("second cancel = %v", err)
	}
	if err := svc.CancelExtraordinary(context.Background(), "syn", condo, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing = %v", err)
	}
}

type failingAudit struct{}

func (failingAudit) RecordEvent(context.Context, *string, model.AuditEventType, map[string]any) error {
	return errors.New("audit down")
}

func TestExtraordinaryAuditFailureIsLogged(t *testing.T) {
	svc, _, _ := extraSvc()
	svc.Audit = failingAudit{}
	id, err := svc.CreateExtraordinary(context.Background(), "syn", condo, extraInput())
	if err != nil {
		t.Fatalf("create must not fail after commit: %v", err)
	}
	if err := svc.CancelExtraordinary(context.Background(), "syn", condo, id); err != nil {
		t.Fatalf("cancel must not fail after commit: %v", err)
	}
}
