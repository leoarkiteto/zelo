package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/core/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/core/ports"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

type fakeAccountStore struct {
	accounts map[string]domain.FinancialAccount
	nextID   int
}

func newFakeAccountStore() *fakeAccountStore {
	return &fakeAccountStore{accounts: map[string]domain.FinancialAccount{}}
}

func (f *fakeAccountStore) Create(_ context.Context, a domain.FinancialAccount) (string, error) {
	f.nextID++
	id := "acc-" + string(rune('0'+f.nextID))
	a.ID = id
	f.accounts[id] = a
	return id, nil
}

func (f *fakeAccountStore) GetByID(_ context.Context, id string) (domain.FinancialAccount, error) {
	a, ok := f.accounts[id]
	if !ok {
		return domain.FinancialAccount{}, model.ErrNotFound
	}
	return a, nil
}

func (f *fakeAccountStore) Update(_ context.Context, a domain.FinancialAccount) error {
	existing, ok := f.accounts[a.ID]
	if !ok || existing.CondominiumID != a.CondominiumID {
		return model.ErrNotFound
	}
	if existing.Status != domain.StatusPending {
		return model.ErrNotFound
	}
	f.accounts[a.ID] = a
	return nil
}

func (f *fakeAccountStore) SetStatus(_ context.Context, id, condominiumID string, status domain.AccountStatus, settlementDate *time.Time) error {
	a, ok := f.accounts[id]
	if !ok || a.CondominiumID != condominiumID || a.Status != domain.StatusPending {
		return model.ErrNotFound
	}
	a.Status = status
	a.SettlementDate = settlementDate
	f.accounts[id] = a
	return nil
}

func (f *fakeAccountStore) List(_ context.Context, _ string, _ ports.AccountFilter) ([]domain.FinancialAccount, error) {
	var out []domain.FinancialAccount
	for _, a := range f.accounts {
		out = append(out, a)
	}
	return out, nil
}

func (f *fakeAccountStore) ListPendingForUnit(_ context.Context, condominiumID, unitID string) ([]domain.FinancialAccount, error) {
	var out []domain.FinancialAccount
	for _, a := range f.accounts {
		if a.CondominiumID == condominiumID && a.UnitID == unitID && a.Status == domain.StatusPending {
			out = append(out, a)
		}
	}
	return out, nil
}

type fakeSummaryStore struct {
	sum ports.MonthSummary
}

func (f *fakeSummaryStore) SummaryForMonth(context.Context, string, time.Time) (ports.MonthSummary, error) {
	return f.sum, nil
}

type fakeUnitStore struct {
	units []model.Unit
	unit  model.Unit
	err   error
}

func (f *fakeUnitStore) ListUnitsForCondominium(context.Context, string) ([]model.Unit, error) {
	return f.units, nil
}

func (f *fakeUnitStore) GetActiveUnitForUser(context.Context, string, string) (model.Unit, error) {
	return f.unit, f.err
}

type fakeAudit struct {
	events []model.AuditEventType
}

func (f *fakeAudit) RecordEvent(_ context.Context, _ *string, eventType model.AuditEventType, _ map[string]any) error {
	f.events = append(f.events, eventType)
	return nil
}

func newService(acc *fakeAccountStore, units *fakeUnitStore, audit *fakeAudit) *FinanceService {
	return &FinanceService{
		Accounts:  acc,
		Summaries: &fakeSummaryStore{},
		Units:     units,
		Audit:     audit,
		Now:       time.Now,
	}
}

func payableInput() AccountInput {
	return AccountInput{
		Type:     domain.AccountTypePayable,
		Title:    "Manutenção do portão",
		Category: domain.CategoryMaintenance,
		Amount:   "350.00",
		DueDate:  "2026-08-30",
	}
}

func TestCreatePayableSuccess(t *testing.T) {
	acc := newFakeAccountStore()
	audit := &fakeAudit{}
	svc := newService(acc, &fakeUnitStore{}, audit)

	got, err := svc.CreateAccount(context.Background(), "u1", "c1", payableInput())
	if err != nil {
		t.Fatalf("CreateAccount() = %v", err)
	}
	if got.AmountCents != 35000 || got.Status != domain.StatusPending {
		t.Fatalf("unexpected account: %+v", got)
	}
	if len(audit.events) != 1 || audit.events[0] != model.AuditAccountCreated {
		t.Fatalf("audit events = %v", audit.events)
	}
}

func TestCreateAccountRejectsBadAmount(t *testing.T) {
	svc := newService(newFakeAccountStore(), &fakeUnitStore{}, &fakeAudit{})
	in := payableInput()
	for _, amount := range []string{"0", "-10", "1.234", "abc"} {
		in.Amount = amount
		if _, err := svc.CreateAccount(context.Background(), "u1", "c1", in); !errors.Is(err, ErrInvalidAmount) {
			t.Errorf("amount %q error = %v, want ErrInvalidAmount", amount, err)
		}
	}
}

func TestCreateAccountRejectsWrongCategory(t *testing.T) {
	svc := newService(newFakeAccountStore(), &fakeUnitStore{}, &fakeAudit{})
	in := payableInput()
	in.Category = domain.CategoryCondoFee // receivable-only
	if _, err := svc.CreateAccount(context.Background(), "u1", "c1", in); !errors.Is(err, ErrInvalidCategory) {
		t.Fatalf("error = %v, want ErrInvalidCategory", err)
	}
}

func TestCreateSettledRequiresDate(t *testing.T) {
	svc := newService(newFakeAccountStore(), &fakeUnitStore{}, &fakeAudit{})
	in := payableInput()
	in.Status = domain.StatusSettled
	in.SettlementDate = ""
	if _, err := svc.CreateAccount(context.Background(), "u1", "c1", in); !errors.Is(err, ErrSettlementDateRequired) {
		t.Fatalf("error = %v, want ErrSettlementDateRequired", err)
	}
}

func TestCreateSettledWithDateSucceeds(t *testing.T) {
	acc := newFakeAccountStore()
	svc := newService(acc, &fakeUnitStore{}, &fakeAudit{})
	in := payableInput()
	in.Status = domain.StatusSettled
	in.SettlementDate = "2026-08-20"
	got, err := svc.CreateAccount(context.Background(), "u1", "c1", in)
	if err != nil {
		t.Fatalf("CreateAccount(settled) = %v", err)
	}
	if got.Status != domain.StatusSettled || got.SettlementDate == nil {
		t.Fatalf("unexpected settled account: %+v", got)
	}
}

func TestCreateReceivableValidatesUnit(t *testing.T) {
	units := &fakeUnitStore{units: []model.Unit{{ID: "u9", CondominiumID: "c1", Code: "A-1"}}}
	svc := newService(newFakeAccountStore(), units, &fakeAudit{})
	in := payableInput()
	in.Type = domain.AccountTypeReceivable
	in.Category = domain.CategoryCondoFee
	in.UnitID = "other-unit"
	if _, err := svc.CreateAccount(context.Background(), "u1", "c1", in); !errors.Is(err, ErrInvalidUnit) {
		t.Fatalf("error = %v, want ErrInvalidUnit", err)
	}
	in.UnitID = "u9"
	if _, err := svc.CreateAccount(context.Background(), "u1", "c1", in); err != nil {
		t.Fatalf("valid unit rejected: %v", err)
	}
}

func TestSettleRequiresDateAndSettles(t *testing.T) {
	acc := newFakeAccountStore()
	audit := &fakeAudit{}
	svc := newService(acc, &fakeUnitStore{}, audit)
	created, _ := svc.CreateAccount(context.Background(), "u1", "c1", payableInput())
	id := created.ID

	if err := svc.SettleAccount(context.Background(), "u1", id, "c1", ""); !errors.Is(err, ErrSettlementDateRequired) {
		t.Fatalf("settle without date = %v, want ErrSettlementDateRequired", err)
	}
	if err := svc.SettleAccount(context.Background(), "u1", id, "c1", "2026-08-22"); err != nil {
		t.Fatalf("settle = %v", err)
	}
	got, _ := acc.GetByID(context.Background(), id)
	if got.Status != domain.StatusSettled || got.SettlementDate == nil {
		t.Fatalf("settled account = %+v", got)
	}
	if len(audit.events) != 2 || audit.events[1] != model.AuditAccountSettled {
		t.Fatalf("audit events = %v", audit.events)
	}
}

func TestSettleAndCancelRejectedWhenReadOnly(t *testing.T) {
	acc := newFakeAccountStore()
	svc := newService(acc, &fakeUnitStore{}, &fakeAudit{})
	created, _ := svc.CreateAccount(context.Background(), "u1", "c1", payableInput())
	id := created.ID
	if err := svc.SettleAccount(context.Background(), "u1", id, "c1", "2026-08-22"); err != nil {
		t.Fatalf("settle = %v", err)
	}
	if err := svc.CancelAccount(context.Background(), "u1", id, "c1"); !errors.Is(err, ErrNotEditable) {
		t.Fatalf("cancel settled = %v, want ErrNotEditable", err)
	}
	if err := svc.EditAccount(context.Background(), "u1", id, "c1", payableInput()); !errors.Is(err, ErrNotEditable) {
		t.Fatalf("edit settled = %v, want ErrNotEditable", err)
	}
}

func TestResidentChargesRequiresUnit(t *testing.T) {
	units := &fakeUnitStore{err: model.ErrNotFound}
	svc := newService(newFakeAccountStore(), units, &fakeAudit{})
	if _, err := svc.ResidentCharges(context.Background(), "u1", "c1"); !errors.Is(err, ErrNoUnit) {
		t.Fatalf("error = %v, want ErrNoUnit", err)
	}
}

func TestSummaryAndHealthPassthrough(t *testing.T) {
	sum := &fakeSummaryStore{sum: ports.MonthSummary{TotalReceivableCents: 100, TotalPayableCents: 40, ProjectedBalanceCents: 60}}
	svc := &FinanceService{Summaries: sum, Now: time.Now}
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	got, err := svc.Summary(context.Background(), "c1", month)
	if err != nil || got.ProjectedBalanceCents != 60 {
		t.Fatalf("Summary() = %+v, %v", got, err)
	}
	if _, err := svc.Health(context.Background(), "c1", month); err != nil {
		t.Fatalf("Health() = %v", err)
	}
}
