package handlers

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/money"
	"github.com/leoarkiteto/zelo/internal/shared/permille"
)

func extraForm() url.Values {
	return url.Values{
		"description": {"Roof"}, "total": {"1200.00"}, "division": {"equal"},
		"installments": {"3"}, "first_due": {"2026-12-10"},
	}
}

func sampleExtraDetail(status domain.IssuanceStatus) services.ExtraordinaryDetail {
	first := time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)
	return services.ExtraordinaryDetail{
		ExtraordinarySummary: services.ExtraordinarySummary{
			ID: "x1", Description: "Roof", TotalCents: 120000, Division: services.DivisionEqual, Installments: 3,
			FirstDueDate: first, Status: status, ChargeCount: 6, SettledCount: 2, SettledInstallments: 1,
		},
		Issuances: []services.IssuanceSummary{
			{ID: "i1", PeriodIndex: 1, Status: status, TotalCents: 40000, ChargeCount: 2, SettledCount: 2},
			{ID: "i2", PeriodIndex: 2, Status: status, TotalCents: 40000, ChargeCount: 2},
		},
	}
}

func TestExtraordinaryRoutesSyndicOnly(t *testing.T) {
	user, sess := syndicSession()
	for _, role := range []model.Role{model.RoleOwner, model.RoleTenant} {
		router := financeRouter(user, sess, quotaDeps(&fakeQuotas{}, role))
		for _, c := range []struct{ method, path string }{
			{"GET", "/finance/extraordinaries"},
			{"GET", "/finance/extraordinaries/new"},
			{"POST", "/finance/extraordinaries/preview"},
			{"POST", "/finance/extraordinaries"},
			{"GET", "/finance/extraordinaries/x1/view"},
			{"GET", "/finance/extraordinaries/x1/cancel"},
			{"POST", "/finance/extraordinaries/x1/cancel"},
		} {
			if rec := do(router, c.method, c.path, url.Values{}); rec.Code != http.StatusForbidden {
				t.Errorf("%s %s as %s = %d, want 403", c.method, c.path, role, rec.Code)
			}
		}
	}
}

func TestExtraordinaryListAndNew(t *testing.T) {
	user, sess := syndicSession()
	q := &fakeQuotas{extras: []services.ExtraordinarySummary{{
		ID: "x1", Description: "Roof", TotalCents: 120000, Installments: 3, Status: domain.IssuanceIssued, SettledInstallments: 1,
	}}}
	router := financeRouter(user, sess, quotaDeps(q, model.RoleSyndic))

	rec := do(router, "GET", "/finance/extraordinaries", nil)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "Roof") || !strings.Contains(body, "/finance/extraordinaries/x1/view") || !strings.Contains(body, "1 / 3") {
		t.Fatalf("list = %d, body missing row: %s", rec.Code, body)
	}
	rec = do(router, "GET", "/finance/extraordinaries/new", nil)
	body = rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `action="/finance/extraordinaries"`) || !strings.Contains(body, `hx-post="/finance/extraordinaries/preview"`) {
		t.Fatalf("new = %d, body missing form/preview wiring", rec.Code)
	}
}

func TestExtraordinaryPreviewFragment(t *testing.T) {
	user, sess := syndicSession()
	due := time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)
	q := &fakeQuotas{extraPreview: services.ExtraordinaryPreview{
		TotalCents: 120000, Division: services.DivisionEqual,
		Installments: []services.ExtraordinaryInstallment{{Number: 1, DueDate: due, TotalCents: 60000}, {Number: 2, DueDate: due.AddDate(0, 1, 0), TotalCents: 60000}},
		Units:        []services.ExtraordinaryUnit{{Code: "A1", SharePPM: permille.Total, PerInstallmentCents: []int64{60000, 60000}, TotalCents: 120000}},
	}}
	router := financeRouter(user, sess, quotaDeps(q, model.RoleSyndic))
	rec := do(router, "POST", "/finance/extraordinaries/preview", extraForm())
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "A1") || !strings.Contains(body, "600.00") || strings.Contains(body, "<html") {
		t.Fatalf("preview = %d, want fragment with unit and amounts: %s", rec.Code, body)
	}
	if q.lastExtra.Installments != 3 || q.lastExtra.TotalCents != 120000 || q.lastExtra.Division != services.DivisionEqual ||
		!q.lastExtra.FirstDueDate.Equal(time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("service input = %+v", q.lastExtra)
	}
}

func TestExtraordinaryErrorsAreTranslated(t *testing.T) {
	user, sess := syndicSession()
	for _, tt := range []struct {
		name   string
		err    error
		status int
		want   string
	}{
		{"installments", domain.ErrInvalidInstallments, http.StatusUnprocessableEntity, "from 1 to 24"},
		{"invalid", domain.ErrInvalidExtraordinary, http.StatusUnprocessableEntity, "first due date"},
		{"shares", services.SharesError{SumPPM: permille.Total - 1000}, http.StatusUnprocessableEntity, "do not add up to 100%"},
	} {
		q := &fakeQuotas{extraErr: tt.err}
		router := financeRouter(user, sess, quotaDeps(q, model.RoleSyndic))
		for _, path := range []string{"/finance/extraordinaries/preview", "/finance/extraordinaries"} {
			rec := do(router, "POST", path, extraForm())
			if rec.Code != tt.status || !strings.Contains(rec.Body.String(), tt.want) {
				t.Errorf("%s %s = %d, want %d containing %q: %s", tt.name, path, rec.Code, tt.status, tt.want, rec.Body.String())
			}
		}
	}
	// A malformed field never reaches the service.
	q := &fakeQuotas{}
	router := financeRouter(user, sess, quotaDeps(q, model.RoleSyndic))
	bad := extraForm()
	bad.Set("installments", "abc")
	if rec := do(router, "POST", "/finance/extraordinaries", bad); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad installments = %d, want 422", rec.Code)
	}
}

func TestExtraordinaryCreateRedirects(t *testing.T) {
	user, sess := syndicSession()
	q := &fakeQuotas{}
	router := financeRouter(user, sess, quotaDeps(q, model.RoleSyndic))
	rec := do(router, "POST", "/finance/extraordinaries", extraForm())
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/finance/extraordinaries/x1/view?flash=extra_created" {
		t.Fatalf("create = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if q.lastExtra.Description != "Roof" {
		t.Fatalf("input = %+v", q.lastExtra)
	}
}

func TestExtraordinaryViewAndCancel(t *testing.T) {
	user, sess := syndicSession()
	q := &fakeQuotas{extraDetail: sampleExtraDetail(domain.IssuanceIssued)}
	router := financeRouter(user, sess, quotaDeps(q, model.RoleSyndic))

	rec := do(router, "GET", "/finance/extraordinaries/x1/view", nil)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "Roof") || !strings.Contains(body, "/finance/extraordinaries/x1/cancel") {
		t.Fatalf("view = %d: %s", rec.Code, body)
	}
	rec = do(router, "GET", "/finance/extraordinaries/x1/cancel", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `action="/finance/extraordinaries/x1/cancel"`) {
		t.Fatalf("cancel page = %d", rec.Code)
	}
	rec = do(router, "POST", "/finance/extraordinaries/x1/cancel", url.Values{})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/finance/extraordinaries?flash=extra_canceled" || q.extraCanceled != "x1" {
		t.Fatalf("cancel = %d %q canceled=%q", rec.Code, rec.Header().Get("Location"), q.extraCanceled)
	}

	q.extraCancelErr = domain.ErrExtraordinaryHasSettled
	rec = do(router, "POST", "/finance/extraordinaries/x1/cancel", url.Values{})
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "already settled") {
		t.Fatalf("refused cancel = %d: %s", rec.Code, rec.Body.String())
	}

	q.extraGetErr = services.ErrNotFound
	if rec = do(router, "GET", "/finance/extraordinaries/x1/view", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("missing view = %d, want 404", rec.Code)
	}
}

// TestExtraordinaryAndLateFeeTitlesPerLocale covers FR-14 on the list,
// Conta corrente and Recibo.
func TestExtraordinaryAndLateFeeTitlesPerLocale(t *testing.T) {
	settled := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	q2 := domain.QuotaPeriod{Year: 2026, Periodicity: domain.PeriodicityQuarterly, Index: 2, UnitCode: "A"}
	inst := domain.QuotaPeriod{Kind: domain.QuotaKindExtraordinary, Description: "Roof", Installment: 2, Installments: 6, UnitCode: "A"}
	for _, tt := range []struct {
		lang, extra, lateFee, label string
	}{
		{"en", "Extraordinary quota — Roof — installment 2/6", "Late fee and interest — Condo fee Q2 2026 — Unit A", "Installment 2/6"},
		{"pt-br", "Quota extraordinária — Roof — parcela 2/6", "Multa e juros — Taxa condominial 2º trimestre de 2026 — Unidade A", "Parcela 2/6"},
		{"pt-pt", "Quota extraordinária — Roof — parcela 2/6", "Multa e juros — Quota 2.º trimestre de 2026 — Unidade A", "Parcela 2/6"},
	} {
		user, sess := syndicSession()
		user.LanguagePreference = tt.lang
		fq := &fakeResidentQuotas{
			statement: services.Statement{
				Unit: model.Unit{ID: "un1", Code: "A"}, Units: []model.Unit{{ID: "un1", Code: "A"}},
				Pending: []services.StatementRow{
					{Account: domain.FinancialAccount{ID: "e1", Title: "stored extra", AmountCents: 100, DueDate: settled, Status: domain.StatusPending}, IsQuota: true, Period: inst},
					{Account: domain.FinancialAccount{ID: "f1", Title: "stored fee", AmountCents: 100, DueDate: settled, Status: domain.StatusPending}, IsLateFee: true, LateFeeOrigin: q2},
				},
			},
		}
		deps := baseDeps(&fakeFinance{
			list: []domain.FinancialAccount{
				{ID: "f1", Type: domain.AccountTypeReceivable, Title: "stored fee", Category: domain.CategoryCondoFee, AmountCents: 100, DueDate: settled, Status: domain.StatusPending},
			},
			lateFees: map[string]domain.QuotaPeriod{"f1": q2},
		})
		deps.Quotas = fq
		router := financeRouter(user, sess, deps)

		check := func(name, target string, wants ...string) {
			t.Helper()
			rec := get(router, target, false)
			body := rec.Body.String()
			if rec.Code != http.StatusOK {
				t.Errorf("%s/%s: code=%d", tt.lang, name, rec.Code)
			}
			for _, w := range wants {
				if !strings.Contains(body, w) {
					t.Errorf("%s/%s: missing %q", tt.lang, name, w)
				}
			}
			for _, absent := range []string{"stored extra", "stored fee"} {
				if strings.Contains(body, absent) {
					t.Errorf("%s/%s: shows stored title %q", tt.lang, name, absent)
				}
			}
		}
		check("list", "/finance", tt.lateFee)
		check("statement", "/finance/my-charges", tt.extra, tt.lateFee)

		fq.receipt = services.ReceiptData{ReceiptRow: services.ReceiptRow{
			AccountID: "e1", Currency: money.EUR, UnitCode: "A", Title: "stored extra", AmountCents: 100, Status: domain.StatusSettled,
			SettlementDate: &settled, IsQuota: true, Period: inst,
		}}
		check("recibo extra", "/finance/e1/recibo", tt.extra, tt.label)
		fq.receipt = services.ReceiptData{ReceiptRow: services.ReceiptRow{
			AccountID: "f1", Currency: money.EUR, UnitCode: "A", Title: "stored fee", AmountCents: 100, Status: domain.StatusSettled,
			SettlementDate: &settled, IsLateFee: true, LateFeeOrigin: q2,
		}}
		check("recibo late fee", "/finance/f1/recibo", tt.lateFee)
	}
}
