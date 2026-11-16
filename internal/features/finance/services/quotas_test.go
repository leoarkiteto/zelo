package services

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/money"
)

// ---- in-memory fakes ----

type recAudit struct {
	types   []model.AuditEventType
	details []map[string]any
}

func (a *recAudit) RecordEvent(_ context.Context, _ *string, t model.AuditEventType, d map[string]any) error {
	a.types = append(a.types, t)
	a.details = append(a.details, d)
	return nil
}

func (a *recAudit) count(t model.AuditEventType) int {
	n := 0
	for _, x := range a.types {
		if x == t {
			n++
		}
	}
	return n
}

type fCharge struct {
	NewCharge
	accountID  string
	issuanceID string
	status     domain.AccountStatus
}

type fIssuance struct {
	IssuanceSummary
}

type qstate struct {
	budgets   map[string]Budget
	lines     map[string][]BudgetLine
	issuances []fIssuance
	charges   []fCharge
	nextID    int
}

func (s qstate) clone() qstate {
	c := qstate{budgets: map[string]Budget{}, lines: map[string][]BudgetLine{}, nextID: s.nextID}
	for k, v := range s.budgets {
		c.budgets[k] = v
	}
	for k, v := range s.lines {
		c.lines[k] = append([]BudgetLine(nil), v...)
	}
	c.issuances = append([]fIssuance(nil), s.issuances...)
	c.charges = append([]fCharge(nil), s.charges...)
	return c
}

// fakeQuotaStore keeps state in memory; With* callbacks run on a snapshot that
// is restored when the callback fails (rollback).
type fakeQuotaStore struct {
	qstate
	units    []domain.ShareInput
	receipts map[string]ReceiptRow
	rows     map[string][]StatementRow // unit id -> receivables
	card     CardRows
	locks    []string
}

func newFakeQuotaStore() *fakeQuotaStore {
	return &fakeQuotaStore{
		qstate:   qstate{budgets: map[string]Budget{}, lines: map[string][]BudgetLine{}},
		receipts: map[string]ReceiptRow{}, rows: map[string][]StatementRow{},
	}
}

func (f *fakeQuotaStore) id(prefix string) string {
	f.nextID++
	return prefix + string(rune('0'+f.nextID))
}

func (f *fakeQuotaStore) tx(fn func() error) error {
	snap := f.qstate.clone()
	if err := fn(); err != nil {
		f.qstate = snap
		return err
	}
	return nil
}

type fBudgetTx struct {
	f  *fakeQuotaStore
	id string
}

func (t *fBudgetTx) Budget() Budget      { return t.f.budgets[t.id] }
func (t *fBudgetTx) Lines() []BudgetLine { return t.f.lines[t.id] }

type fLockedBudget struct{ *fBudgetTx }

func (l fLockedBudget) UpdateParams(p BudgetParams) error {
	b := l.f.budgets[l.id]
	b.Periodicity, b.DueDay, b.ReserveRateBP = p.Periodicity, p.DueDay, p.ReserveRateBP
	l.f.budgets[l.id] = b
	return nil
}

func (l fLockedBudget) AddLine(ln BudgetLine) (string, error) {
	ln.ID = l.f.id("l")
	ln.BudgetID = l.id
	l.f.lines[l.id] = append(l.f.lines[l.id], ln)
	return ln.ID, nil
}

func (l fLockedBudget) UpdateLine(ln BudgetLine) error {
	for i, x := range l.f.lines[l.id] {
		if x.ID == ln.ID {
			l.f.lines[l.id][i] = ln
			return nil
		}
	}
	return model.ErrNotFound
}

func (l fLockedBudget) RemoveLine(id string) error {
	ls := l.f.lines[l.id]
	for i, x := range ls {
		if x.ID == id {
			l.f.lines[l.id] = append(append([]BudgetLine(nil), ls[:i]...), ls[i+1:]...)
			return nil
		}
	}
	return model.ErrNotFound
}

func (l fLockedBudget) SetStatus(s domain.BudgetStatus) error {
	b := l.f.budgets[l.id]
	b.Status = s
	l.f.budgets[l.id] = b
	return nil
}

func (l fLockedBudget) HasActiveIssuance() (bool, error) {
	for _, i := range l.f.issuances {
		if i.BudgetID == l.id && i.Status == domain.IssuanceIssued {
			return true, nil
		}
	}
	return false, nil
}

type fLockedIssue struct{ *fBudgetTx }

func (l fLockedIssue) Shares() ([]domain.ShareInput, error) { return l.f.units, nil }

func (l fLockedIssue) InsertIssuance(in NewIssuance) (string, bool, error) {
	for _, i := range l.f.issuances {
		if i.BudgetID == l.id && i.PeriodIndex == in.PeriodIndex && i.Status == domain.IssuanceIssued {
			return i.ID, false, nil
		}
	}
	b := l.f.budgets[l.id]
	iss := IssuanceSummary{
		ID: l.f.id("i"), BudgetID: l.id, Year: b.Year, Periodicity: b.Periodicity, PeriodIndex: in.PeriodIndex,
		Status: domain.IssuanceIssued, TotalCents: in.TotalCents, BaseCents: in.BaseCents, ReserveCents: in.ReserveCents,
	}
	l.f.issuances = append(l.f.issuances, fIssuance{iss})
	return iss.ID, true, nil
}

func (l fLockedIssue) InsertCharges(issuanceID string, cs []NewCharge) error {
	for _, c := range cs {
		l.f.charges = append(l.f.charges, fCharge{NewCharge: c, accountID: l.f.id("a"), issuanceID: issuanceID, status: domain.StatusPending})
	}
	return nil
}

func (f *fakeQuotaStore) WithLockedBudget(_ context.Context, condoID, budgetID string, fn func(context.Context, LockedBudget) error) error {
	b, ok := f.budgets[budgetID]
	if !ok || b.CondominiumID != condoID {
		return model.ErrNotFound
	}
	f.locks = append(f.locks, "budget")
	return f.tx(func() error { return fn(context.Background(), fLockedBudget{&fBudgetTx{f, budgetID}}) })
}

func (f *fakeQuotaStore) WithLockedIssue(_ context.Context, condoID, budgetID string, fn func(context.Context, LockedIssue) error) error {
	b, ok := f.budgets[budgetID]
	if !ok || b.CondominiumID != condoID {
		return model.ErrNotFound
	}
	f.locks = append(f.locks, "budget")
	return f.tx(func() error { return fn(context.Background(), fLockedIssue{&fBudgetTx{f, budgetID}}) })
}

type fLockedIssuance struct {
	f  *fakeQuotaStore
	id string
}

func (l fLockedIssuance) idx() int {
	for i, x := range l.f.issuances {
		if x.ID == l.id {
			return i
		}
	}
	return -1
}

func (l fLockedIssuance) Issuance() IssuanceSummary { return l.f.issuances[l.idx()].IssuanceSummary }

func (l fLockedIssuance) CancelPendingCharges() (int, int, error) {
	total, affected := 0, 0
	for i, c := range l.f.charges {
		if c.issuanceID != l.id {
			continue
		}
		total++
		if c.status == domain.StatusPending {
			l.f.charges[i].status = domain.StatusCanceled
			affected++
		}
	}
	return affected, total, nil
}

func (l fLockedIssuance) MarkCanceled(at time.Time) error {
	i := l.idx()
	l.f.issuances[i].Status = domain.IssuanceCanceled
	l.f.issuances[i].CanceledAt = &at
	return nil
}

func (f *fakeQuotaStore) WithLockedIssuance(_ context.Context, _, issuanceID string, fn func(context.Context, LockedIssuance) error) error {
	li := fLockedIssuance{f, issuanceID}
	if li.idx() < 0 {
		return model.ErrNotFound
	}
	f.locks = append(f.locks, "issuance")
	return f.tx(func() error { return fn(context.Background(), li) })
}

func (f *fakeQuotaStore) CreateBudget(_ context.Context, b Budget) (string, error) {
	for _, x := range f.budgets {
		if x.CondominiumID == b.CondominiumID && x.Year == b.Year {
			return "", domain.ErrBudgetExists
		}
	}
	b.ID = f.id("b")
	if b.Status == "" {
		b.Status = domain.BudgetDraft
	}
	f.budgets[b.ID] = b
	return b.ID, nil
}

func (f *fakeQuotaStore) ListBudgets(context.Context, string) ([]BudgetSummary, error) {
	return nil, nil
}

func (f *fakeQuotaStore) GetBudget(_ context.Context, condoID, id string) (BudgetDetail, error) {
	b, ok := f.budgets[id]
	if !ok || b.CondominiumID != condoID {
		return BudgetDetail{}, model.ErrNotFound
	}
	return BudgetDetail{Budget: b, Lines: f.lines[id]}, nil
}

func (f *fakeQuotaStore) ListIssuances(context.Context, string, string) ([]IssuanceSummary, error) {
	return nil, nil
}

func (f *fakeQuotaStore) ActiveIssuanceFor(_ context.Context, _, budgetID string, period int) (IssuanceSummary, bool, error) {
	for _, i := range f.issuances {
		if i.BudgetID == budgetID && i.PeriodIndex == period && i.Status == domain.IssuanceIssued {
			return i.IssuanceSummary, true, nil
		}
	}
	return IssuanceSummary{}, false, nil
}

func (f *fakeQuotaStore) UnitReceivables(_ context.Context, _, unitID string) ([]StatementRow, error) {
	return f.rows[unitID], nil
}

func (f *fakeQuotaStore) ReceiptData(_ context.Context, _, accountID string) (ReceiptRow, error) {
	r, ok := f.receipts[accountID]
	if !ok {
		return ReceiptRow{}, model.ErrNotFound
	}
	return r, nil
}

func (f *fakeQuotaStore) CardData(context.Context, string, []string, time.Time) (CardRows, error) {
	return f.card, nil
}

// ListUnitsForCondominium makes the fake store double as the shared UnitStore.
func (f *fakeQuotaStore) ListUnitsForCondominium(_ context.Context, condoID string) ([]model.Unit, error) {
	var out []model.Unit
	for _, u := range f.units {
		out = append(out, model.Unit{ID: u.UnitID, CondominiumID: condoID, Code: u.Code, SharePPM: u.SharePPM})
	}
	return out, nil
}

type fakeOcc struct{ units []model.ActiveUnit }

func (f fakeOcc) ListActiveOccupanciesForUser(context.Context, string, string) ([]model.ActiveUnit, error) {
	return f.units, nil
}

func (f *fakeQuotaStore) chargesOf(issuanceID string) []fCharge {
	var out []fCharge
	for _, c := range f.charges {
		if c.issuanceID == issuanceID {
			out = append(out, c)
		}
	}
	return out
}

// ---- helpers ----

const condo = "c1"

func newQuotaSvc(cur money.Currency) (*QuotaService, *fakeQuotaStore, *recAudit) {
	st := newFakeQuotaStore()
	au := &recAudit{}
	return &QuotaService{
		Store: st, Units: st, Occupancies: fakeOcc{}, Audit: au,
		Currency: func(context.Context, string) (money.Currency, error) { return cur, nil },
		Now:      func() time.Time { return time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC) },
	}, st, au
}

// approvedBudget seeds an approved monthly budget of 1200.00 (120000 cents),
// reserve 10%, due day 5, with three units 50/30/20 percent.
func approvedBudget(st *fakeQuotaStore) string {
	st.budgets["b0"] = Budget{ID: "b0", CondominiumID: condo, Year: 2026, Status: domain.BudgetApproved,
		Periodicity: domain.PeriodicityMonthly, DueDay: 5, ReserveRateBP: 1000}
	st.lines["b0"] = []BudgetLine{{ID: "l0", BudgetID: "b0", Description: "x", Category: domain.CategoryMaintenance, AmountCents: 120000}}
	st.units = []domain.ShareInput{
		{UnitID: "u1", Code: "A-1", SharePPM: 500000},
		{UnitID: "u2", Code: "A-2", SharePPM: 300000},
		{UnitID: "u3", Code: "A-3", SharePPM: 200000},
	}
	return "b0"
}

// ---- issuance ----

func TestIssueHappyPathAndIdempotency(t *testing.T) {
	svc, st, au := newQuotaSvc(money.EUR)
	b := approvedBudget(st)
	id, created, err := svc.Issue(context.Background(), "syn", condo, b, 3)
	if err != nil || !created {
		t.Fatalf("Issue = %q, %v, %v", id, created, err)
	}
	cs := st.chargesOf(id)
	if len(cs) != 3 {
		t.Fatalf("charges = %d, want 3", len(cs))
	}
	want := map[string]int64{"u1": 5500, "u2": 3300, "u3": 2200} // base 10000 + 10% reserve
	var sum int64
	for _, c := range cs {
		sum += c.AmountCents
		if c.AmountCents != want[c.UnitID] || c.BaseCents+c.ReserveCents != c.AmountCents {
			t.Errorf("charge %+v, want amount %d", c, want[c.UnitID])
		}
		if !c.DueDate.Equal(time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("due = %v", c.DueDate)
		}
		if c.SharePPM == 0 || c.Title == "" {
			t.Errorf("missing frozen share/title: %+v", c)
		}
	}
	if sum != 11000 {
		t.Fatalf("sum = %d, want 11000", sum)
	}
	if au.count(model.AuditQuotaIssued) != 1 {
		t.Fatalf("audit = %v", au.types)
	}

	// Shares change afterwards (FR-17) and the period is requested again.
	st.units[0].SharePPM, st.units[1].SharePPM = 400000, 400000
	id2, created2, err := svc.Issue(context.Background(), "syn", condo, b, 3)
	if err != nil || created2 || id2 != id {
		t.Fatalf("re-issue = %q, %v, %v; want %q false nil", id2, created2, err, id)
	}
	if len(st.charges) != 3 || au.count(model.AuditQuotaIssued) != 1 {
		t.Fatalf("re-issue created data: charges=%d audit=%v", len(st.charges), au.types)
	}
	for _, c := range st.charges {
		if c.SharePPM == 400000 {
			t.Fatalf("issued quota changed with the unit share: %+v", c)
		}
	}
}

func TestIssueLockOrder(t *testing.T) {
	svc, st, _ := newQuotaSvc(money.EUR)
	b := approvedBudget(st)
	id, _, _ := svc.Issue(context.Background(), "syn", condo, b, 1)
	if err := svc.CancelIssuance(context.Background(), "syn", condo, id); err != nil {
		t.Fatal(err)
	}
	want := []string{"budget", "issuance"}
	if len(st.locks) != 2 || st.locks[0] != want[0] || st.locks[1] != want[1] {
		t.Fatalf("locks = %v, want %v (issue locks the budget, cancel only the issuance)", st.locks, want)
	}
}

func TestIssueRefusals(t *testing.T) {
	ctx := context.Background()
	t.Run("shares not 100", func(t *testing.T) {
		svc, st, _ := newQuotaSvc(money.EUR)
		b := approvedBudget(st)
		st.units[2].SharePPM = 150000 // sum 95%
		_, _, err := svc.Issue(ctx, "s", condo, b, 1)
		var se SharesError
		if !errors.Is(err, domain.ErrSharesNot100) || !errors.As(err, &se) || se.SumPPM != 950000 || se.DiffPPM() != -50000 {
			t.Fatalf("err = %v (%+v)", err, se)
		}
		if len(st.issuances) != 0 {
			t.Fatal("issuance created on refusal")
		}
	})
	t.Run("draft budget", func(t *testing.T) {
		svc, st, _ := newQuotaSvc(money.EUR)
		b := approvedBudget(st)
		bb := st.budgets[b]
		bb.Status = domain.BudgetDraft
		st.budgets[b] = bb
		if _, _, err := svc.Issue(ctx, "s", condo, b, 1); !errors.Is(err, domain.ErrBudgetNotApproved) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("period out of range", func(t *testing.T) {
		svc, st, _ := newQuotaSvc(money.EUR)
		b := approvedBudget(st)
		for _, p := range []int{0, 13} {
			if _, _, err := svc.Issue(ctx, "s", condo, b, p); !errors.Is(err, domain.ErrPeriodOutOfRange) {
				t.Fatalf("period %d err = %v", p, err)
			}
		}
	})
	t.Run("no chargeable units", func(t *testing.T) {
		svc, st, _ := newQuotaSvc(money.BRL)
		b := approvedBudget(st)
		st.lines[b] = []BudgetLine{{AmountCents: 1}} // 1 cent over 12 periods: period 12 is 0
		if _, _, err := svc.Issue(ctx, "s", condo, b, 12); !errors.Is(err, domain.ErrNoChargeableUnits) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("unknown budget", func(t *testing.T) {
		svc, _, _ := newQuotaSvc(money.BRL)
		if _, _, err := svc.Issue(ctx, "s", condo, "nope", 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestIssueSkipsZeroShareAndZeroQuotaUnits(t *testing.T) {
	svc, st, _ := newQuotaSvc(money.BRL)
	b := approvedBudget(st)
	st.units = []domain.ShareInput{
		{UnitID: "u1", Code: "A-1", SharePPM: 600000},
		{UnitID: "u2", Code: "A-2", SharePPM: 400000},
		{UnitID: "z", Code: "Z-0", SharePPM: 0},
	}
	st.lines[b] = []BudgetLine{{AmountCents: 1}} // period 1 gets 1 cent: only the larger share receives it
	id, _, err := svc.Issue(context.Background(), "s", condo, b, 1)
	if err != nil {
		t.Fatal(err)
	}
	cs := st.chargesOf(id)
	if len(cs) != 1 || cs[0].UnitID != "u1" || cs[0].AmountCents != 1 {
		t.Fatalf("charges = %+v", cs)
	}
}

func TestPreviewIssuance(t *testing.T) {
	svc, st, _ := newQuotaSvc(money.EUR)
	b := approvedBudget(st)
	pv, err := svc.PreviewIssuance(context.Background(), condo, b, 2)
	if err != nil {
		t.Fatal(err)
	}
	if pv.TotalCents != 11000 || pv.BaseCents != 10000 || pv.ReserveCents != 1000 || len(pv.Rows) != 3 || pv.Label != "2026-02" {
		t.Fatalf("preview = %+v", pv)
	}
	id, _, _ := svc.Issue(context.Background(), "s", condo, b, 2)
	pv, err = svc.PreviewIssuance(context.Background(), condo, b, 2)
	if err != nil || pv.Existing == nil || pv.Existing.ID != id {
		t.Fatalf("preview of issued period = %+v, %v", pv, err)
	}
}

func TestIssueIsAtomicOnFailure(t *testing.T) {
	svc, st, _ := newQuotaSvc(money.EUR)
	b := approvedBudget(st)
	// A failing InsertCharges rolls back the issuance (fake snapshot restore).
	boom := errors.New("boom")
	err := st.WithLockedIssue(context.Background(), condo, b, func(ctx context.Context, l LockedIssue) error {
		if _, _, err := l.InsertIssuance(NewIssuance{BudgetID: b, PeriodIndex: 1}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) || len(st.issuances) != 0 {
		t.Fatalf("err=%v issuances=%d", err, len(st.issuances))
	}
	if _, created, err := svc.Issue(context.Background(), "s", condo, b, 1); err != nil || !created {
		t.Fatalf("issue after rollback: %v %v", created, err)
	}
}

// ---- cancel ----

func TestCancelIssuance(t *testing.T) {
	svc, st, au := newQuotaSvc(money.EUR)
	b := approvedBudget(st)
	id, _, _ := svc.Issue(context.Background(), "s", condo, b, 1)
	if err := svc.CancelIssuance(context.Background(), "s", condo, id); err != nil {
		t.Fatal(err)
	}
	for _, c := range st.chargesOf(id) {
		if c.status != domain.StatusCanceled {
			t.Fatalf("charge not canceled: %+v", c)
		}
	}
	if au.count(model.AuditQuotaIssuanceCanceled) != 1 {
		t.Fatalf("audit = %v", au.types)
	}
	// The period can be issued again, and canceling twice is refused.
	if _, created, err := svc.Issue(context.Background(), "s", condo, b, 1); err != nil || !created {
		t.Fatalf("re-issue after cancel: %v %v", created, err)
	}
	if err := svc.CancelIssuance(context.Background(), "s", condo, id); !errors.Is(err, ErrIssuanceNotActive) {
		t.Fatalf("second cancel = %v", err)
	}
}

func TestCancelIssuanceWithSettledChargeRollsBack(t *testing.T) {
	svc, st, au := newQuotaSvc(money.EUR)
	b := approvedBudget(st)
	id, _, _ := svc.Issue(context.Background(), "s", condo, b, 1)
	st.charges[1].status = domain.StatusSettled
	err := svc.CancelIssuance(context.Background(), "s", condo, id)
	if !errors.Is(err, domain.ErrIssuanceHasSettled) {
		t.Fatalf("err = %v", err)
	}
	for i, c := range st.chargesOf(id) {
		want := domain.StatusPending
		if i == 1 {
			want = domain.StatusSettled
		}
		if c.status != want {
			t.Fatalf("charge %d status = %s, want %s (rolled back)", i, c.status, want)
		}
	}
	if st.issuances[0].Status != domain.IssuanceIssued || au.count(model.AuditQuotaIssuanceCanceled) != 0 {
		t.Fatal("issuance state or audit changed")
	}
}

// ---- budgets ----

func TestCreateBudgetDefaultsPerCurrency(t *testing.T) {
	for cur, want := range map[money.Currency]int{money.EUR: 1000, money.BRL: 0} {
		svc, st, au := newQuotaSvc(cur)
		b, err := svc.CreateBudget(context.Background(), "s", condo, BudgetInput{Year: 2026, Periodicity: domain.PeriodicityMonthly, DueDay: 10})
		if err != nil || b.ReserveRateBP != want || st.budgets[b.ID].ReserveRateBP != want {
			t.Fatalf("%s: %+v %v, want rate %d", cur, b, err, want)
		}
		if au.count(model.AuditBudgetCreated) != 1 {
			t.Fatalf("audit = %v", au.types)
		}
	}
	svc, _, _ := newQuotaSvc(money.EUR)
	rate := 250
	b, err := svc.CreateBudget(context.Background(), "s", condo, BudgetInput{Year: 2026, Periodicity: domain.PeriodicityQuarterly, DueDay: 1, ReserveRateBP: &rate})
	if err != nil || b.ReserveRateBP != 250 {
		t.Fatalf("explicit rate: %+v %v", b, err)
	}
}

func TestCreateBudgetRules(t *testing.T) {
	svc, _, _ := newQuotaSvc(money.EUR)
	ctx := context.Background()
	in := BudgetInput{Year: 2026, Periodicity: domain.PeriodicityMonthly, DueDay: 10}
	if _, err := svc.CreateBudget(ctx, "s", condo, in); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateBudget(ctx, "s", condo, in); !errors.Is(err, domain.ErrBudgetExists) {
		t.Fatalf("duplicate year err = %v", err)
	}
	for _, bad := range []BudgetInput{
		{Year: 2027, Periodicity: domain.PeriodicityMonthly, DueDay: 0},
		{Year: 2027, Periodicity: domain.PeriodicityMonthly, DueDay: 29},
		{Year: 2027, Periodicity: "weekly", DueDay: 5},
	} {
		if _, err := svc.CreateBudget(ctx, "s", condo, bad); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%+v err = %v", bad, err)
		}
	}
}

func TestBudgetEditApproveReopen(t *testing.T) {
	svc, st, au := newQuotaSvc(money.EUR)
	ctx := context.Background()
	b, _ := svc.CreateBudget(ctx, "s", condo, BudgetInput{Year: 2026, Periodicity: domain.PeriodicityMonthly, DueDay: 10})

	if err := svc.ApproveBudget(ctx, "s", condo, b.ID); !errors.Is(err, domain.ErrBudgetEmpty) {
		t.Fatalf("approve empty = %v", err)
	}
	if _, err := svc.AddLine(ctx, "s", condo, b.ID, LineInput{Description: "", Category: domain.CategoryCleaning, AmountCents: 100}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank line = %v", err)
	}
	if _, err := svc.AddLine(ctx, "s", condo, b.ID, LineInput{Description: "x", Category: domain.CategoryCondoFee, AmountCents: 100}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("receivable category = %v", err)
	}
	if _, err := svc.AddLine(ctx, "s", condo, b.ID, LineInput{Description: "x", Category: domain.CategoryCleaning, AmountCents: 0}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("zero amount = %v", err)
	}
	lid, err := svc.AddLine(ctx, "s", condo, b.ID, LineInput{Description: "Limpeza", Category: domain.CategoryCleaning, AmountCents: 120000})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateLine(ctx, "s", condo, b.ID, lid, LineInput{Description: "Limpeza", Category: domain.CategoryCleaning, AmountCents: 130000}); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateBudget(ctx, "s", condo, b.ID, BudgetParams{Periodicity: domain.PeriodicityQuarterly, DueDay: 15, ReserveRateBP: 500}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApproveBudget(ctx, "s", condo, b.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApproveBudget(ctx, "s", condo, b.ID); !errors.Is(err, domain.ErrBudgetLocked) {
		t.Fatalf("approve twice = %v", err)
	}
	// Approved budgets accept no edits.
	for name, err := range map[string]error{
		"params": svc.UpdateBudget(ctx, "s", condo, b.ID, BudgetParams{Periodicity: domain.PeriodicityMonthly, DueDay: 1}),
		"update": svc.UpdateLine(ctx, "s", condo, b.ID, lid, LineInput{Description: "y", Category: domain.CategoryCleaning, AmountCents: 1}),
		"remove": svc.RemoveLine(ctx, "s", condo, b.ID, lid),
	} {
		if !errors.Is(err, domain.ErrBudgetLocked) {
			t.Errorf("%s on approved = %v", name, err)
		}
	}
	if _, err := svc.AddLine(ctx, "s", condo, b.ID, LineInput{Description: "z", Category: domain.CategoryCleaning, AmountCents: 1}); !errors.Is(err, domain.ErrBudgetLocked) {
		t.Fatalf("add on approved = %v", err)
	}

	// Reopen is refused with an active issuance and allowed after it is canceled.
	id, _, err := svc.Issue(ctx, "s", condo, b.ID, 1)
	st.units = []domain.ShareInput{{UnitID: "u1", Code: "A-1", SharePPM: 1000000}}
	if err != nil && !errors.Is(err, domain.ErrSharesNot100) {
		t.Fatal(err)
	}
	id, _, err = svc.Issue(ctx, "s", condo, b.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ReopenBudget(ctx, "s", condo, b.ID); !errors.Is(err, domain.ErrBudgetHasIssuances) {
		t.Fatalf("reopen with issuance = %v", err)
	}
	if err := svc.CancelIssuance(ctx, "s", condo, id); err != nil {
		t.Fatal(err)
	}
	if err := svc.ReopenBudget(ctx, "s", condo, b.ID); err != nil {
		t.Fatalf("reopen = %v", err)
	}
	if err := svc.ReopenBudget(ctx, "s", condo, b.ID); !errors.Is(err, domain.ErrBudgetNotApproved) {
		t.Fatalf("reopen draft = %v", err)
	}
	if err := svc.RemoveLine(ctx, "s", condo, b.ID, lid); err != nil {
		t.Fatal(err)
	}
	if au.count(model.AuditBudgetApproved) != 1 || au.count(model.AuditBudgetReopened) != 1 || au.count(model.AuditBudgetUpdated) < 3 {
		t.Fatalf("audit = %v", au.types)
	}
}

func TestGetBudgetPreview(t *testing.T) {
	svc, st, _ := newQuotaSvc(money.EUR)
	b := approvedBudget(st)
	d, err := svc.GetBudget(context.Background(), condo, b)
	if err != nil {
		t.Fatal(err)
	}
	if d.TotalCents != 120000 || len(d.Periods) != 12 || len(d.Units) != 3 || d.SumPPM != 1000000 {
		t.Fatalf("detail = %+v", d)
	}
	if d.Units[0].PerPeriodCents[0] != 5500 || d.Periods[0].TotalCents != 11000 {
		t.Fatalf("preview = %+v / %+v", d.Units[0], d.Periods[0])
	}
}

// ---- statement ----

func row(id string, status domain.AccountStatus, due string, settled string) StatementRow {
	d, _ := time.Parse("2006-01-02", due)
	a := domain.FinancialAccount{ID: id, UnitID: "u1", Status: status, DueDate: d, AmountCents: 1000, Type: domain.AccountTypeReceivable}
	if settled != "" {
		s, _ := time.Parse("2006-01-02", settled)
		a.SettlementDate = &s
	}
	return StatementRow{Account: a}
}

func statementFixture() (*QuotaService, *fakeQuotaStore, *recAudit) {
	svc, st, au := newQuotaSvc(money.EUR)
	st.units = []domain.ShareInput{{UnitID: "u1", Code: "A-1"}, {UnitID: "u2", Code: "A-2"}}
	started := time.Date(2026, 3, 10, 23, 30, 0, 0, time.UTC)
	svc.Occupancies = fakeOcc{units: []model.ActiveUnit{
		{Unit: model.Unit{ID: "u1", Code: "A-1"}, OccupancyType: model.OccupancyTenant, StartedAt: started},
		{Unit: model.Unit{ID: "u3", Code: "B-1"}, OccupancyType: model.OccupancyOwner, StartedAt: started},
	}}
	st.rows["u1"] = []StatementRow{
		row("p-old", domain.StatusPending, "2026-01-05", ""),
		row("p-new", domain.StatusPending, "2026-04-05", ""),
		row("s-before", domain.StatusSettled, "2026-02-05", "2026-03-09"),
		row("s-sameday", domain.StatusSettled, "2026-02-05", "2026-03-10"),
		row("s-after", domain.StatusSettled, "2026-02-05", "2026-03-11"),
		row("c1", domain.StatusCanceled, "2026-02-05", ""),
	}
	return svc, st, au
}

func ids(rows []StatementRow) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Account.ID)
	}
	sort.Strings(out)
	return out
}

func TestStatementOccupantVisibility(t *testing.T) {
	svc, _, _ := statementFixture()
	st, err := svc.Statement(context.Background(), Viewer{UserID: "me"}, condo, "")
	if err != nil {
		t.Fatal(err)
	}
	if st.Unit.ID != "u1" || len(st.Units) != 2 {
		t.Fatalf("unit = %+v units=%v", st.Unit, st.Units)
	}
	if got := ids(st.Pending); len(got) != 2 {
		t.Fatalf("pending = %v (all pending are visible)", got)
	}
	got := ids(st.Settled)
	if len(got) != 2 || got[0] != "s-after" || got[1] != "s-sameday" {
		t.Fatalf("settled = %v, want the start day and later only", got)
	}
	if len(st.Canceled) != 0 {
		t.Fatal("occupant must not see canceled")
	}
	if st.PendingTotalCents != 2000 || st.OverdueCount != 1 || st.OverdueTotalCents != 1000 {
		t.Fatalf("totals = %+v", st)
	}
}

func TestStatementForbiddenAndSyndic(t *testing.T) {
	svc, _, au := statementFixture()
	ctx := context.Background()
	if _, err := svc.Statement(ctx, Viewer{UserID: "me"}, condo, "u2"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("other unit = %v", err)
	}
	if au.count(model.AuditAccessDenied) != 1 || au.details[0]["unit_id"] != "u2" {
		t.Fatalf("audit = %v %v", au.types, au.details)
	}
	st, err := svc.Statement(ctx, Viewer{UserID: "syn", IsSyndic: true}, condo, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Settled) != 3 || len(st.Canceled) != 1 || len(st.Units) != 2 {
		t.Fatalf("syndic sees everything: %d settled, %d canceled", len(st.Settled), len(st.Canceled))
	}
	if _, err := svc.Statement(ctx, Viewer{UserID: "syn", IsSyndic: true}, condo, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown unit = %v", err)
	}
	svc.Occupancies = fakeOcc{}
	if _, err := svc.Statement(ctx, Viewer{UserID: "x"}, condo, ""); !errors.Is(err, ErrNoUnit) {
		t.Fatalf("no unit = %v", err)
	}
}

// ---- receipt ----

func TestReceipt(t *testing.T) {
	svc, st, au := statementFixture()
	ctx := context.Background()
	d := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	before := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	st.receipts["abcdef12-0000-0000-0000-000000000000"] = ReceiptRow{AccountID: "abcdef12-0000-0000-0000-000000000000", UnitID: "u1", UnitCode: "A-1", Status: domain.StatusSettled, SettlementDate: &d, AmountCents: 5}
	st.receipts["old"] = ReceiptRow{AccountID: "old", UnitID: "u1", Status: domain.StatusSettled, SettlementDate: &before}
	st.receipts["pend"] = ReceiptRow{AccountID: "pend", UnitID: "u1", Status: domain.StatusPending}
	st.receipts["pend-other"] = ReceiptRow{AccountID: "pend-other", UnitID: "u2", Status: domain.StatusPending}
	st.receipts["other"] = ReceiptRow{AccountID: "other", UnitID: "u2", Status: domain.StatusSettled, SettlementDate: &d}
	st.receipts["nounit"] = ReceiptRow{AccountID: "nounit", Status: domain.StatusSettled, SettlementDate: &d}
	me, syn := Viewer{UserID: "me"}, Viewer{UserID: "syn", IsSyndic: true}

	r, err := svc.Receipt(ctx, me, condo, "abcdef12-0000-0000-0000-000000000000")
	if err != nil || r.Number != "ABCDEF12" || r.UnitCode != "A-1" {
		t.Fatalf("own receipt = %+v, %v", r, err)
	}
	if _, err := svc.Receipt(ctx, me, condo, "pend"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("pending own = %v, want not found", err)
	}
	if _, err := svc.Receipt(ctx, syn, condo, "pend"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("pending syndic = %v, want not found", err)
	}
	if _, err := svc.Receipt(ctx, me, condo, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing = %v", err)
	}
	if au.count(model.AuditAccessDenied) != 0 {
		t.Fatal("no denial expected yet")
	}
	for _, id := range []string{"other", "pend-other", "nounit", "old"} {
		if _, err := svc.Receipt(ctx, me, condo, id); !errors.Is(err, ErrForbidden) {
			t.Fatalf("%s = %v, want forbidden", id, err)
		}
	}
	if au.count(model.AuditAccessDenied) != 4 || au.details[0]["account_id"] != "other" {
		t.Fatalf("audit = %v %v", au.types, au.details)
	}
	for _, id := range []string{"other", "old", "nounit"} {
		if _, err := svc.Receipt(ctx, syn, condo, id); err != nil {
			t.Fatalf("syndic %s = %v", id, err)
		}
	}
}

// ---- card ----

func TestCardResidentAndSyndic(t *testing.T) {
	svc, st, _ := statementFixture()
	ctx := context.Background()
	today := time.Date(2026, 5, 20, 8, 0, 0, 0, time.UTC)
	next := &CardCharge{AmountCents: 1234, DueDate: time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)}
	st.card = CardRows{NextPending: next, OverdueCount: 2, OverdueTotalCents: 900,
		Budget: &Budget{ID: "b9", Year: 2026, Status: domain.BudgetApproved, Periodicity: domain.PeriodicityQuarterly}}

	c, err := svc.Card(ctx, Viewer{UserID: "me"}, condo, today)
	if err != nil || c.Syndic != nil || c.Resident == nil || c.Resident.NextPending != next || c.Resident.OverdueCount != 2 || c.Resident.OverdueTotalCents != 900 {
		t.Fatalf("resident card = %+v, %v", c, err)
	}

	// Syndic without units; period of 2026-05-20 in a quarterly budget is Q2.
	svc.Occupancies = fakeOcc{}
	c, err = svc.Card(ctx, Viewer{UserID: "syn", IsSyndic: true}, condo, today)
	if err != nil || c.Resident != nil || c.Syndic == nil {
		t.Fatalf("syndic card = %+v, %v", c, err)
	}
	if !c.Syndic.HasBudget || c.Syndic.PeriodIndex != 2 || c.Syndic.Issued {
		t.Fatalf("not issued state = %+v", c.Syndic)
	}
	st.issuances = []fIssuance{{IssuanceSummary{ID: "i1", BudgetID: "b9", PeriodIndex: 2, Status: domain.IssuanceIssued, ChargeCount: 10, SettledCount: 4}}}
	c, _ = svc.Card(ctx, Viewer{UserID: "syn", IsSyndic: true}, condo, today)
	if !c.Syndic.Issued || c.Syndic.ChargeCount != 10 || c.Syndic.SettledCount != 4 {
		t.Fatalf("issued state = %+v", c.Syndic)
	}

	st.card.Budget = nil
	c, _ = svc.Card(ctx, Viewer{UserID: "syn", IsSyndic: true}, condo, today)
	if c.Syndic.HasBudget {
		t.Fatalf("no approved budget = %+v", c.Syndic)
	}
	if c, _ := svc.Card(ctx, Viewer{UserID: "nobody"}, condo, today); c.Resident != nil || c.Syndic != nil {
		t.Fatalf("outsider card = %+v", c)
	}
}
