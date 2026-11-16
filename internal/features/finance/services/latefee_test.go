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

// ---- fake additions to fakeQuotaStore ----

type fLateFee struct {
	NewLateFee
	quotaID   string
	accountID string
}

type fLockedQuotaAccount struct {
	f   *fakeQuotaStore
	acc QuotaAccount
}

func (l *fLockedQuotaAccount) Account() QuotaAccount { return l.acc }

func (l *fLockedQuotaAccount) Settle(time.Time) error {
	if l.acc.Status != domain.StatusPending {
		return domain.ErrNotEditable
	}
	l.acc.Status = domain.StatusSettled
	l.f.qaccts[l.acc.ID] = l.acc
	return nil
}

func (l *fLockedQuotaAccount) InsertLateFee(in NewLateFee) (string, error) {
	for _, lf := range l.f.lateFees {
		if lf.quotaID == l.acc.ID {
			return "", domain.ErrNotEditable // UNIQUE quota_account_id
		}
	}
	id := l.f.id("late")
	l.f.lateFees = append(l.f.lateFees, fLateFee{NewLateFee: in, quotaID: l.acc.ID, accountID: id})
	return id, nil
}

func (f *fakeQuotaStore) QuotaAccount(_ context.Context, condoID, id string) (QuotaAccount, error) {
	a, ok := f.qaccts[id]
	if !ok || a.CondominiumID != condoID {
		return QuotaAccount{}, model.ErrNotFound
	}
	return a, nil
}

func (f *fakeQuotaStore) WithLockedQuotaAccount(_ context.Context, condoID, id string, fn func(context.Context, LockedQuotaAccount) error) error {
	a, ok := f.qaccts[id]
	if !ok || a.CondominiumID != condoID {
		return model.ErrNotFound
	}
	f.locks = append(f.locks, "account")
	if f.onAccountLock != nil {
		f.onAccountLock()
	}
	accts := map[string]QuotaAccount{}
	for k, v := range f.qaccts {
		accts[k] = v
	}
	fees := append([]fLateFee(nil), f.lateFees...)
	if err := fn(context.Background(), &fLockedQuotaAccount{f: f, acc: a}); err != nil {
		f.qaccts, f.lateFees = accts, fees // rollback
		return err
	}
	return nil
}

func (f *fakeQuotaStore) DebtorRows(context.Context, string, time.Time) ([]DebtorRow, error) {
	f.debtorCalls++
	return f.debtors, nil
}

func (f *fakeQuotaStore) OverdueQuotaRows(context.Context, string, time.Time) ([]OverdueQuota, error) {
	f.overdueCalls++
	return f.overdue, nil
}

func (f *fakeQuotaStore) ReserveTotals(context.Context, string, int) (ReserveTotals, error) {
	return f.reserve, nil
}

// ---- helpers ----

var (
	lateDue    = time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	lateSettle = time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC) // 10 days late
)

// lateSvc returns a service with a pending quota "q1" of 1000.00 due on 5 March
// and the settings multa 2 %, juros 1 %/month, no grace.
func lateSvc(cur money.Currency) (*QuotaService, *fakeQuotaStore, *recAudit) {
	svc, st, au := newQuotaSvc(cur)
	st.qaccts = map[string]QuotaAccount{
		"q1":   {ID: "q1", CondominiumID: condo, UnitID: "u1", Title: "Quota 2026-03 A-1", AmountCents: 100000, DueDate: lateDue, Status: domain.StatusPending, IsQuota: true},
		"free": {ID: "free", CondominiumID: condo, UnitID: "u1", Title: "Outra", AmountCents: 100000, DueDate: lateDue, Status: domain.StatusPending},
	}
	svc.LateFees = func(context.Context, string) (model.LateFeeSettings, error) {
		return model.LateFeeSettings{FeeBP: 200, InterestBP: 100}, nil
	}
	return svc, st, au
}

// ---- LateFeePreview / SettleQuota ----

func TestLateFeePreview(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := lateSvc(money.BRL)
	q, err := svc.LateFeePreview(ctx, condo, "q1", lateSettle)
	// fine 2000, interest 100000*100*10/300000 = 333.33 -> 333
	if err != nil || q.FineCents != 2000 || q.InterestCents != 333 || q.TotalCents != 2333 || q.DaysLate != 10 || q.FeeBP != 200 {
		t.Fatalf("preview = %+v, %v", q, err)
	}
	if q, _ := svc.LateFeePreview(ctx, condo, "q1", lateDue); q.TotalCents != 0 {
		t.Fatalf("on the due date: %+v, want no late fee", q)
	}
	if q, _ := svc.LateFeePreview(ctx, condo, "free", lateSettle); q.TotalCents != 0 {
		t.Fatalf("plain receivable: %+v, want no late fee", q)
	}
	if _, err := svc.LateFeePreview(ctx, condo, "nope", lateSettle); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing account err = %v", err)
	}
}

func TestSettleQuotaDecisionRequired(t *testing.T) {
	ctx := context.Background()
	svc, st, au := lateSvc(money.BRL)
	err := svc.SettleQuota(ctx, "syn", condo, "q1", lateSettle, "")
	if !errors.Is(err, ErrLateFeeDecisionRequired) {
		t.Fatalf("err = %v, want ErrLateFeeDecisionRequired", err)
	}
	if st.qaccts["q1"].Status != domain.StatusPending || len(st.lateFees) != 0 || len(au.types) != 0 {
		t.Fatal("nothing may change without a decision")
	}
	if err := svc.SettleQuota(ctx, "syn", condo, "q1", lateSettle, "maybe"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid decision err = %v", err)
	}
	if err := svc.SettleQuota(ctx, "syn", condo, "q1", time.Time{}, "apply"); !errors.Is(err, ErrSettlementDateRequired) {
		t.Fatalf("zero date err = %v", err)
	}
}

func TestSettleQuotaApply(t *testing.T) {
	ctx := context.Background()
	svc, st, au := lateSvc(money.BRL)
	svc.LateFees = func(context.Context, string) (model.LateFeeSettings, error) {
		return model.LateFeeSettings{FeeBP: 500, InterestBP: 100, GraceDays: 0}, nil // above the BRL cap
	}
	if err := svc.SettleQuota(ctx, "syn", condo, "q1", lateSettle.Add(7*time.Hour), "apply"); err != nil {
		t.Fatal(err)
	}
	if st.qaccts["q1"].Status != domain.StatusSettled {
		t.Fatal("quota not settled")
	}
	if len(st.lateFees) != 1 {
		t.Fatalf("late fees = %d, want 1", len(st.lateFees))
	}
	lf := st.lateFees[0]
	d := lf.Detail
	if lf.quotaID != "q1" || d.FineCents != 2000 || d.InterestCents != 333 || d.DaysLate != 10 || d.FeeBP != 200 || d.InterestBP != 100 {
		t.Fatalf("late fee = %+v (effective multa bp must be the clamped 200)", lf)
	}
	if lf.Title != "Multa e juros — Quota 2026-03 A-1" {
		t.Fatalf("stored late fee title = %q", lf.Title)
	}
	if !lf.DueDate.Equal(lateSettle) || lf.CreatedBy != "syn" {
		t.Fatalf("late fee receivable = %+v", lf.NewLateFee)
	}
	if au.count(model.AuditAccountSettled) != 1 || au.count(model.AuditLateFeeApplied) != 1 || au.count(model.AuditLateFeeWaived) != 0 {
		t.Fatalf("audit = %v", au.types)
	}
	// A second settlement of the same quota is refused and creates nothing.
	if err := svc.SettleQuota(ctx, "syn", condo, "q1", lateSettle, "apply"); !errors.Is(err, ErrNotEditable) {
		t.Fatalf("second settle err = %v, want ErrNotEditable", err)
	}
	if len(st.lateFees) != 1 {
		t.Fatal("second settle created another late fee")
	}
}

func TestSettleQuotaEURIsNotClamped(t *testing.T) {
	svc, st, _ := lateSvc(money.EUR)
	svc.LateFees = func(context.Context, string) (model.LateFeeSettings, error) {
		return model.LateFeeSettings{FeeBP: 500}, nil
	}
	if err := svc.SettleQuota(context.Background(), "syn", condo, "q1", lateSettle, "apply"); err != nil {
		t.Fatal(err)
	}
	if st.lateFees[0].Detail.FeeBP != 500 || st.lateFees[0].Detail.FineCents != 5000 {
		t.Fatalf("EUR late fee = %+v", st.lateFees[0].Detail)
	}
}

func TestSettleQuotaWaive(t *testing.T) {
	svc, st, au := lateSvc(money.BRL)
	if err := svc.SettleQuota(context.Background(), "syn", condo, "q1", lateSettle, "waive"); err != nil {
		t.Fatal(err)
	}
	if st.qaccts["q1"].Status != domain.StatusSettled || len(st.lateFees) != 0 {
		t.Fatal("waive must settle without a late fee")
	}
	if au.count(model.AuditAccountSettled) != 1 || au.count(model.AuditLateFeeWaived) != 1 || au.count(model.AuditLateFeeApplied) != 0 {
		t.Fatalf("audit = %v", au.types)
	}
	for i, ty := range au.types {
		if ty == model.AuditLateFeeWaived && au.details[i]["amount_cents"] != int64(2333) {
			t.Fatalf("waiver details = %v, want the waived amount 2333", au.details[i])
		}
	}
}

func TestSettleQuotaNoLateFee(t *testing.T) {
	ctx := context.Background()
	t.Run("on time needs no decision", func(t *testing.T) {
		svc, st, au := lateSvc(money.BRL)
		if err := svc.SettleQuota(ctx, "syn", condo, "q1", lateDue, ""); err != nil {
			t.Fatal(err)
		}
		if st.qaccts["q1"].Status != domain.StatusSettled || len(st.lateFees) != 0 ||
			au.count(model.AuditAccountSettled) != 1 || au.count(model.AuditLateFeeWaived) != 0 {
			t.Fatalf("state %v audit %v", st.qaccts["q1"], au.types)
		}
	})
	t.Run("grace period", func(t *testing.T) {
		svc, st, _ := lateSvc(money.BRL)
		svc.LateFees = func(context.Context, string) (model.LateFeeSettings, error) {
			return model.LateFeeSettings{FeeBP: 200, InterestBP: 100, GraceDays: 10}, nil
		}
		if err := svc.SettleQuota(ctx, "syn", condo, "q1", lateSettle, ""); err != nil || len(st.lateFees) != 0 {
			t.Fatalf("within grace: err=%v fees=%d", err, len(st.lateFees))
		}
	})
	t.Run("LateFees nil means zero rates", func(t *testing.T) {
		svc, st, _ := lateSvc(money.BRL)
		svc.LateFees = nil
		if err := svc.SettleQuota(ctx, "syn", condo, "q1", lateSettle, ""); err != nil || st.qaccts["q1"].Status != domain.StatusSettled {
			t.Fatalf("nil LateFees: err=%v", err)
		}
		if q, err := svc.LateFeePreview(ctx, condo, "q1", lateSettle); err != nil || q.TotalCents != 0 {
			t.Fatalf("preview = %+v, %v", q, err)
		}
	})
}

func TestSettleQuotaRefusals(t *testing.T) {
	ctx := context.Background()
	svc, st, _ := lateSvc(money.BRL)
	if err := svc.SettleQuota(ctx, "syn", condo, "free", lateSettle, "apply"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("plain receivable err = %v, want ErrNotFound (never a late fee on a non-quota)", err)
	}
	if err := svc.SettleQuota(ctx, "syn", condo, "nope", lateSettle, "apply"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing err = %v", err)
	}
	if err := svc.SettleQuota(ctx, "syn", "other", "q1", lateSettle, "apply"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other condominium err = %v", err)
	}
	a := st.qaccts["q1"]
	a.Status = domain.StatusCanceled
	st.qaccts["q1"] = a
	if err := svc.SettleQuota(ctx, "syn", condo, "q1", lateSettle, "apply"); !errors.Is(err, ErrNotEditable) {
		t.Fatalf("canceled err = %v, want ErrNotEditable", err)
	}
}

func TestSettleQuotaRollsBackOnLateFeeConflict(t *testing.T) {
	ctx := context.Background()
	svc, st, au := lateSvc(money.BRL)
	// A late fee already exists for q1 (UNIQUE quota_account_id) although the quota is still pending.
	st.lateFees = []fLateFee{{quotaID: "q1", accountID: "x"}}
	if err := svc.SettleQuota(ctx, "syn", condo, "q1", lateSettle, "apply"); !errors.Is(err, ErrNotEditable) {
		t.Fatalf("err = %v, want ErrNotEditable", err)
	}
	if st.qaccts["q1"].Status != domain.StatusPending || len(st.lateFees) != 1 || len(au.types) != 0 {
		t.Fatal("settlement was not rolled back")
	}
}

type downAudit struct{ calls int }

func (a *downAudit) RecordEvent(context.Context, *string, model.AuditEventType, map[string]any) error {
	a.calls++
	return errors.New("audit down")
}

func TestSettleQuotaAuditFailureAfterCommitIsNotReturned(t *testing.T) {
	svc, st, _ := lateSvc(money.BRL)
	fa := &downAudit{}
	svc.Audit = fa
	if err := svc.SettleQuota(context.Background(), "syn", condo, "q1", lateSettle, "apply"); err != nil {
		t.Fatalf("a committed settlement must not fail on audit: %v", err)
	}
	if st.qaccts["q1"].Status != domain.StatusSettled || len(st.lateFees) != 1 || fa.calls != 2 {
		t.Fatalf("settled=%v fees=%d audit calls=%d", st.qaccts["q1"].Status, len(st.lateFees), fa.calls)
	}
}

func TestSettleQuotaLocksOnlyTheAccount(t *testing.T) {
	svc, st, _ := lateSvc(money.BRL)
	_ = svc.SettleQuota(context.Background(), "syn", condo, "q1", lateDue, "")
	if len(st.locks) != 1 || st.locks[0] != "account" {
		t.Fatalf("locks = %v, want only the account row (no budget/issuance/extraordinary)", st.locks)
	}
}

func TestSettleQuotaResolvesRatesBeforeTheLock(t *testing.T) {
	svc, st, _ := lateSvc(money.BRL)
	var order []string
	svc.LateFees = func(context.Context, string) (model.LateFeeSettings, error) {
		order = append(order, "settings")
		return model.LateFeeSettings{FeeBP: 200, InterestBP: 100}, nil
	}
	svc.Currency = func(context.Context, string) (money.Currency, error) {
		order = append(order, "currency")
		return money.BRL, nil
	}
	st.onAccountLock = func() { order = append(order, "lock") }
	if err := svc.SettleQuota(context.Background(), "syn", condo, "q1", lateSettle, "apply"); err != nil {
		t.Fatal(err)
	}
	if len(order) != 3 || order[0] != "settings" || order[1] != "currency" || order[2] != "lock" {
		t.Fatalf("call order = %v, want settings, currency, then lock (no reads inside the lock)", order)
	}
}

// ---- debtors ----

func TestDebtorsEstimateAndNoNPlusOne(t *testing.T) {
	ctx := context.Background()
	svc, st, _ := lateSvc(money.BRL)
	today := lateSettle
	st.debtors = []DebtorRow{
		{UnitID: "u1", UnitCode: "A-1", OverdueCount: 2, OverdueTotalCents: 150000, OldestDue: lateDue,
			Occupants: []DebtorOccupant{{Email: "a@x", Type: model.OccupancyOwner}, {Email: "b@x", Type: model.OccupancyTenant}}},
		{UnitID: "u2", UnitCode: "A-2", OverdueCount: 1, OverdueTotalCents: 50000, OldestDue: lateDue.AddDate(0, 0, 5)},
	}
	st.overdue = []OverdueQuota{
		{UnitID: "u1", AmountCents: 100000, DueDate: lateDue}, // 2333
		// the second overdue receivable of u1 is not a quota, so it adds nothing
	}
	got, err := svc.Debtors(ctx, condo, today.Add(5*time.Hour))
	if err != nil || len(got) != 2 {
		t.Fatalf("debtors = %+v, %v", got, err)
	}
	if got[0].UnitCode != "A-1" || got[0].EstimatedLateFeeCents != 2333 || got[0].DaysLate != 10 || len(got[0].Occupants) != 2 {
		t.Fatalf("first debtor = %+v", got[0])
	}
	if got[1].EstimatedLateFeeCents != 0 || got[1].DaysLate != 5 {
		t.Fatalf("second debtor = %+v", got[1])
	}
	if st.debtorCalls != 1 || st.overdueCalls != 1 {
		t.Fatalf("queries = %d + %d, want exactly one of each regardless of the unit count", st.debtorCalls, st.overdueCalls)
	}

	st.debtors = nil
	st.debtorCalls, st.overdueCalls = 0, 0
	if got, err := svc.Debtors(ctx, condo, today); err != nil || len(got) != 0 || st.overdueCalls != 0 {
		t.Fatalf("no debtors: %v %v overdueCalls=%d", got, err, st.overdueCalls)
	}
}

// ---- reserve ----

func TestReserveBalance(t *testing.T) {
	svc, st, _ := lateSvc(money.BRL)
	st.reserve = ReserveTotals{SettledCents: 7000, PendingCents: 3000, SettledYearCents: 4000}
	got, err := svc.ReserveBalance(context.Background(), condo, lateSettle)
	if err != nil || got.RealizedCents != 7000 || got.ForecastCents != 10000 || got.RealizedYearCents != 4000 {
		t.Fatalf("reserve = %+v, %v", got, err)
	}
}

// ---- card ----

func TestCardSyndicDebtors(t *testing.T) {
	svc, st, _ := lateSvc(money.BRL)
	st.card = CardRows{DebtorUnits: 2, DebtorTotalCents: 340000}
	c, err := svc.Card(context.Background(), Viewer{UserID: "s", IsSyndic: true}, condo, lateSettle)
	if err != nil || c.Syndic == nil || c.Syndic.DebtorUnits != 2 || c.Syndic.DebtorTotalCents != 340000 {
		t.Fatalf("card = %+v, %v", c.Syndic, err)
	}
}

func TestCardIncludesDebtorsOnlyForSyndic(t *testing.T) {
	svc, st, _ := lateSvc(money.BRL)
	svc.Occupancies = fakeOcc{units: []model.ActiveUnit{{Unit: model.Unit{ID: "u1"}}}}
	if _, err := svc.Card(context.Background(), Viewer{UserID: "r"}, condo, lateSettle); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Card(context.Background(), Viewer{UserID: "s", IsSyndic: true}, condo, lateSettle); err != nil {
		t.Fatal(err)
	}
	if len(st.cardIncludeDebtors) != 2 || st.cardIncludeDebtors[0] || !st.cardIncludeDebtors[1] {
		t.Fatalf("includeDebtors flags = %v, want [false true]", st.cardIncludeDebtors)
	}
}

// ---- FinanceService late fee links ----

type fakeLateFeeLinks struct {
	ids   map[string]domain.LateFeeOrigin
	calls int
}

func (f *fakeLateFeeLinks) LateFeeOrigins(_ context.Context, ids []string) (map[string]domain.LateFeeOrigin, error) {
	f.calls++
	out := map[string]domain.LateFeeOrigin{}
	for _, id := range ids {
		if o, ok := f.ids[id]; ok {
			out[id] = o
		}
	}
	return out, nil
}

func TestLateFeeAccountsInGenericFlows(t *testing.T) {
	ctx := context.Background()
	acc := newFakeAccountStore()
	seedReceivable(acc, "late")
	seedReceivable(acc, "free")
	audit := &recAudit{}
	svc := newService(acc, &fakeUnitStore{}, &fakeAudit{})
	svc.Audit = audit
	origin := domain.LateFeeOrigin{
		Origin: domain.QuotaPeriod{Year: 2026, Periodicity: domain.PeriodicityMonthly, Index: 3, UnitCode: "A"},
		Detail: domain.LateFeeDetail{FineCents: 2000, InterestCents: 333, DaysLate: 10},
	}
	links := &fakeLateFeeLinks{ids: map[string]domain.LateFeeOrigin{"late": origin}}

	// nil LateFeeLinks: no late fees, nothing breaks.
	if late, err := svc.IsLateFeeAccount(ctx, "late"); late || err != nil {
		t.Fatalf("nil links: %v %v", late, err)
	}
	svc.LateFeeLinks = links

	rows, err := svc.ListAccountsWithFlags(ctx, "c1", AccountFilter{})
	if err != nil || links.calls != 1 {
		t.Fatalf("rows=%d calls=%d err=%v", len(rows), links.calls, err)
	}
	for _, r := range rows {
		if r.IsLateFee != (r.Account.ID == "late") {
			t.Fatalf("late fee flag wrong for %s", r.Account.ID)
		}
		if r.IsLateFee && (r.LateFeeOrigin.Origin.Index != 3 || r.LateFeeOrigin.Detail.DaysLate != 10) {
			t.Fatalf("origin = %+v", r.LateFeeOrigin)
		}
	}

	in := AccountInput{Title: "x", Category: domain.CategoryFineInterest, Amount: "10.00", DueDate: "2026-08-30"}
	if err := svc.EditAccount(ctx, "u1", "late", "c1", in); !errors.Is(err, ErrQuotaManaged) {
		t.Fatalf("EditAccount(late fee) = %v, want ErrQuotaManaged", err)
	}
	if err := svc.CancelAccount(ctx, "u1", "late", "c1"); err != nil {
		t.Fatalf("CancelAccount(late fee) = %v", err)
	}
	if acc.accounts["late"].Status != domain.StatusCanceled {
		t.Fatal("late fee not canceled")
	}
	if audit.count(model.AuditAccountCanceled) != 1 || audit.count(model.AuditLateFeeWaived) != 1 {
		t.Fatalf("audit = %v, want account_canceled and late_fee_waived", audit.types)
	}
	// A plain receivable cancel is not a waiver.
	if err := svc.CancelAccount(ctx, "u1", "free", "c1"); err != nil || audit.count(model.AuditLateFeeWaived) != 1 {
		t.Fatalf("plain cancel: %v audit=%v", err, audit.types)
	}
}
