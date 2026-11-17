package handlers

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

var paginationHrefRe = regexp.MustCompile(`href="(/finance/[^"]*(?:_page|page)=[^"]*)"`)

func paginationHrefs(body string) []url.Values {
	var out []url.Values
	for _, m := range paginationHrefRe.FindAllStringSubmatch(body, -1) {
		u, err := url.Parse(html.UnescapeString(m[1]))
		if err != nil {
			continue
		}
		out = append(out, u.Query())
	}
	return out
}

func statementRowsOf(prefix string, n int, settled bool) []services.StatementRow {
	day := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	rows := make([]services.StatementRow, 0, n)
	for i := 0; i < n; i++ {
		a := domain.FinancialAccount{ID: fmt.Sprintf("%s%02d", prefix, i), Title: fmt.Sprintf("%s item %02d", prefix, i), AmountCents: 100, DueDate: day.Add(24 * time.Hour), Status: domain.StatusPending}
		if settled {
			a.SettlementDate, a.Status = &day, domain.StatusSettled
		}
		rows = append(rows, services.StatementRow{Account: a})
	}
	return rows
}

func TestStatementTablesPaginateIndependently(t *testing.T) {
	st := services.Statement{
		Unit:     model.Unit{ID: "un1", Code: "101"},
		Units:    []model.Unit{{ID: "un1", Code: "101"}, {ID: "un2", Code: "102"}},
		Pending:  statementRowsOf("pnd", 3, false),
		Settled:  statementRowsOf("stl", 45, true),
		Canceled: statementRowsOf("cnc", 25, false),

		PendingTotalCents: 300, OverdueCount: 0,
	}
	router, _ := residentRouter(&fakeResidentQuotas{statement: st}, model.RoleTenant)

	rec := get(router, "/finance/my-charges?settled_page=2&canceled_page=2", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d", rec.Code)
	}
	body := rec.Body.String()
	if n := strings.Count(body, "/recibo"); n != 20 {
		t.Errorf("settled page 2 rows = %d, want 20", n)
	}
	if !strings.Contains(body, "stl item 20") || strings.Contains(body, "stl item 19") || strings.Contains(body, "stl item 40") {
		t.Errorf("settled page 2 must show items 20..39")
	}
	if !strings.Contains(body, "pnd item 02") {
		t.Errorf("pending table must be untouched")
	}
	if !strings.Contains(body, "cnc item 20") || !strings.Contains(body, "cnc item 24") || strings.Contains(body, "cnc item 19") {
		t.Errorf("canceled page 2 must show items 20..24")
	}
	if !strings.Contains(body, "R$ 3.00") {
		t.Errorf("totals must cover the full set")
	}

	var next url.Values
	for _, q := range paginationHrefs(body) {
		if q.Get("settled_page") == "3" {
			next = q
		}
	}
	if next == nil {
		t.Fatalf("no next link for the settled table in %v", paginationHrefs(body))
	}
	if next.Get("unit") != "un1" || next.Get("canceled_page") != "2" {
		t.Errorf("settled next link = %v, want unit=un1 and canceled_page=2 kept", next)
	}
	if next.Has("pending_page") {
		t.Errorf("unrequested pending_page must not appear: %v", next)
	}
}

func TestStatementWithoutCanceledHasNoCanceledNav(t *testing.T) {
	st := sampleStatement()
	router, _ := residentRouter(&fakeResidentQuotas{statement: st}, model.RoleTenant)
	body := get(router, "/finance/my-charges", false).Body.String()
	if strings.Contains(body, "page=") {
		t.Errorf("short lists must render no pagination links")
	}
}

func TestBudgetsListPaginatesInMemory(t *testing.T) {
	var budgets []services.BudgetSummary
	for i := 0; i < 45; i++ {
		budgets = append(budgets, services.BudgetSummary{Budget: services.Budget{ID: fmt.Sprintf("b%02d", i), Year: 2000 + i, Periodicity: domain.PeriodicityMonthly}})
	}
	user, sess := syndicSession()
	router := financeRouter(user, sess, quotaDeps(&fakeQuotas{budgets: budgets}, model.RoleSyndic))

	for _, tt := range []struct {
		target string
		rows   int
		prev   bool
		next   bool
	}{
		{"/finance/budgets", 20, false, true},
		{"/finance/budgets?page=2", 20, true, true},
		{"/finance/budgets?page=3", 5, true, false},
		{"/finance/budgets?page=99", 5, true, false},
		{"/finance/budgets?page=abc", 20, false, true},
	} {
		rec := get(router, tt.target, false)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d", tt.target, rec.Code)
		}
		body := rec.Body.String()
		if n := strings.Count(body, `/view"`); n != tt.rows {
			t.Errorf("%s rows = %d, want %d", tt.target, n, tt.rows)
		}
		hasPrev := strings.Contains(body, ">Previous<")
		hasNext := strings.Contains(body, ">Next<")
		if hasPrev != tt.prev || hasNext != tt.next {
			t.Errorf("%s prev=%v next=%v, want %v/%v", tt.target, hasPrev, hasNext, tt.prev, tt.next)
		}
	}
}
