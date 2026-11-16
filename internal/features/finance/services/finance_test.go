package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
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

func (f *fakeAccountStore) List(_ context.Context, _ string, _ AccountFilter) ([]domain.FinancialAccount, error) {
	var out []domain.FinancialAccount
	for _, a := range f.accounts {
		out = append(out, a)
	}
	return out, nil
}

type fakeSummaryStore struct {
	sum MonthSummary
}

func (f *fakeSummaryStore) SummaryForMonth(context.Context, string, time.Time) (MonthSummary, error) {
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

func TestSummaryPassthrough(t *testing.T) {
	sum := &fakeSummaryStore{sum: MonthSummary{TotalReceivableCents: 100, TotalPayableCents: 40, ProjectedBalanceCents: 60}}
	svc := &FinanceService{Summaries: sum, Now: time.Now}
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	got, err := svc.Summary(context.Background(), "c1", month)
	if err != nil || got.ProjectedBalanceCents != 60 {
		t.Fatalf("Summary() = %+v, %v", got, err)
	}
}

type fakeQuotaLinks struct {
	ids   map[string]bool
	calls int
	err   error
}

func (f *fakeQuotaLinks) QuotaPeriods(_ context.Context, ids []string) (map[string]domain.QuotaPeriod, error) {
	f.calls++
	out := map[string]domain.QuotaPeriod{}
	for _, id := range ids {
		if f.ids[id] {
			out[id] = domain.QuotaPeriod{Year: 2026, Periodicity: domain.PeriodicityMonthly, Index: 8, UnitCode: "A"}
		}
	}
	return out, f.err
}

func TestQuotaPeriodSingleAccount(t *testing.T) {
	ctx := context.Background()
	svc := newService(newFakeAccountStore(), &fakeUnitStore{}, &fakeAudit{})
	if _, ok, err := svc.QuotaPeriod(ctx, "q1"); ok || err != nil {
		t.Fatalf("QuotaPeriod without QuotaLinks = ok %v, err %v; want false, nil", ok, err)
	}

	links := &fakeQuotaLinks{ids: map[string]bool{"q1": true}}
	svc.QuotaLinks = links
	p, ok, err := svc.QuotaPeriod(ctx, "q1")
	if err != nil || !ok || p.Index != 8 || p.UnitCode != "A" {
		t.Fatalf("QuotaPeriod(quota) = %+v, %v, %v", p, ok, err)
	}
	if _, ok, err := svc.QuotaPeriod(ctx, "free"); ok || err != nil {
		t.Fatalf("QuotaPeriod(non-quota) = ok %v, err %v; want false, nil", ok, err)
	}
	if links.calls != 2 {
		t.Fatalf("QuotaPeriods calls = %d, want one per lookup", links.calls)
	}

	links.err = errors.New("boom")
	if _, _, err := svc.QuotaPeriod(ctx, "q1"); err == nil {
		t.Fatal("QuotaPeriod did not return the lookup error")
	}
}

func seedReceivable(acc *fakeAccountStore, id string) {
	acc.accounts[id] = domain.FinancialAccount{
		ID: id, CondominiumID: "c1", Type: domain.AccountTypeReceivable, Title: "Quota",
		Category: domain.CategoryCondoFee, AmountCents: 1000, Status: domain.StatusPending,
		DueDate: time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC),
	}
}

func TestQuotaManagedAccountsAreLocked(t *testing.T) {
	acc := newFakeAccountStore()
	seedReceivable(acc, "q1")
	seedReceivable(acc, "free")
	audit := &fakeAudit{}
	svc := newService(acc, &fakeUnitStore{}, audit)
	svc.QuotaLinks = &fakeQuotaLinks{ids: map[string]bool{"q1": true}}
	ctx := context.Background()

	in := AccountInput{Title: "Quota", Category: domain.CategoryCondoFee, Amount: "10.00", DueDate: "2026-08-30"}
	if err := svc.EditAccount(ctx, "u1", "q1", "c1", in); !errors.Is(err, ErrQuotaManaged) {
		t.Fatalf("EditAccount(quota) = %v, want ErrQuotaManaged", err)
	}
	if err := svc.CancelAccount(ctx, "u1", "q1", "c1"); !errors.Is(err, ErrQuotaManaged) {
		t.Fatalf("CancelAccount(quota) = %v, want ErrQuotaManaged", err)
	}
	if acc.accounts["q1"].Status != domain.StatusPending || len(audit.events) != 0 {
		t.Fatal("quota account changed or audited")
	}
	// Settlement stays available through the generic flow.
	if err := svc.SettleAccount(ctx, "u1", "q1", "c1", "2026-08-20"); err != nil {
		t.Fatalf("SettleAccount(quota) = %v", err)
	}
	// A non-quota receivable can still be canceled.
	if err := svc.CancelAccount(ctx, "u1", "free", "c1"); err != nil {
		t.Fatalf("CancelAccount(free) = %v", err)
	}
}

func TestListAccountsWithFlagsUsesOneBatchLookup(t *testing.T) {
	acc := newFakeAccountStore()
	seedReceivable(acc, "q1")
	seedReceivable(acc, "free")
	svc := newService(acc, &fakeUnitStore{}, &fakeAudit{})
	links := &fakeQuotaLinks{ids: map[string]bool{"q1": true}}
	svc.QuotaLinks = links
	rows, err := svc.ListAccountsWithFlags(context.Background(), "c1", AccountFilter{})
	if err != nil || len(rows) != 2 || links.calls != 1 {
		t.Fatalf("rows=%d calls=%d err=%v", len(rows), links.calls, err)
	}
	for _, r := range rows {
		if r.IsQuota != (r.Account.ID == "q1") {
			t.Fatalf("flag wrong for %s", r.Account.ID)
		}
		if r.IsQuota && (r.Period.Index != 8 || r.Period.UnitCode != "A" || r.Period.Year != 2026) {
			t.Fatalf("quota metadata missing: %+v", r.Period)
		}
	}
	// nil QuotaLinks: no flags, no lookup.
	svc.QuotaLinks = nil
	rows, err = svc.ListAccountsWithFlags(context.Background(), "c1", AccountFilter{})
	if err != nil || len(rows) != 2 || rows[0].IsQuota || rows[1].IsQuota {
		t.Fatalf("nil links rows=%+v err=%v", rows, err)
	}
}
