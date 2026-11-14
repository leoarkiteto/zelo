package integration

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/security"
)

const (
	memberPass = "member-pass-123"

	secretTitle    = "ZZ-SECRET-TITLE-9431"
	secretSupplier = "ZZ-SECRET-SUPPLIER-7720"
)

// financeEnv is a running composed app with finance fixtures loaded.
type financeEnv struct {
	base      string
	db        *sql.DB
	condoID   string
	otherCond string
	syndicID  string
	now       time.Time
}

// get issues a GET with optional headers and returns status, headers and body.
func get(t *testing.T, client *http.Client, url string, hdr map[string]string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build get: %v", err)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(b)
}

func loginAs(t *testing.T, base, email, password string) *http.Client {
	t.Helper()
	c := newClient(t)
	code, _ := postForm(t, c, base, "/login", map[string]string{
		"email": email, "password": password,
		"csrf_token": csrfFrom(t, c, base, "/login"),
	})
	if code != http.StatusSeeOther {
		t.Fatalf("login %s = %d, want 303", email, code)
	}
	return c
}

func dateStr(d time.Time) string { return d.Format("2006-01-02") }

// addAccount inserts one finance_accounts row. settled == "" means no
// settlement date. supplier is only valid for payables.
func (e *financeEnv) addAccount(t *testing.T, condoID, typ, category, title string, cents int64, due time.Time, status string, settled string, supplier string) {
	t.Helper()
	var settledArg, supplierArg any
	if settled != "" {
		settledArg = settled
	}
	if supplier != "" {
		supplierArg = supplier
	}
	if _, err := e.db.ExecContext(context.Background(), `
		INSERT INTO finance_accounts
			(condominium_id, account_type, title, category, amount_cents, due_date, status, settlement_date, supplier_payee, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		condoID, typ, title, category, cents, dateStr(due), status, settledArg, supplierArg, e.syndicID); err != nil {
		t.Fatalf("insert account %s: %v", title, err)
	}
}

func insertUserWithRole(t *testing.T, db *sql.DB, email, role, condoID string) {
	t.Helper()
	hash, err := security.NewPasswordHasher("integration-test-pepper").Hash(memberPass)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	var id string
	if err := db.QueryRowContext(context.Background(),
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, email, hash).Scan(&id); err != nil {
		t.Fatalf("insert user %s: %v", email, err)
	}
	if role == "" {
		return
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO user_roles (user_id, condominium_id, role) VALUES ($1, $2, $3)`, id, condoID, role); err != nil {
		t.Fatalf("insert role %s: %v", role, err)
	}
}

// Fixture amounts in cents, relative to now (UTC):
//
//	settled receivables: this month 100000, month-1 25050, month-2 7000
//	  (the last is due in month-3 and settled in month-2)
//	settled payables:    this month 30075, month-1 4525
//	pending receivable due in 20 days (55500), overdue pending receivable (12345)
//	canceled payable (88800)
//
// Totals: in 132050, out 34600, balance 97450, overdue 12345.
func newFinanceEnv(t *testing.T) *financeEnv {
	t.Helper()
	router, db := newAppDB(t)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)

	e := &financeEnv{base: ts.URL, db: db, condoID: os.Getenv("TEST_CONDOMINIUM"), now: time.Now().UTC()}
	ctx := context.Background()

	if err := db.QueryRowContext(ctx, `SELECT id FROM users WHERE email='syndic@example.com'`).Scan(&e.syndicID); err != nil {
		t.Fatalf("syndic id: %v", err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO condominiums (name) VALUES ('Other Condo') RETURNING id`).Scan(&e.otherCond); err != nil {
		t.Fatalf("other condo: %v", err)
	}
	insertUserWithRole(t, db, "tenant-fin@example.com", "tenant", e.condoID)
	insertUserWithRole(t, db, "norole@example.com", "", "")

	first := time.Date(e.now.Year(), e.now.Month(), 1, 0, 0, 0, 0, time.UTC)
	m1 := first.AddDate(0, -1, 0)
	m2 := first.AddDate(0, -2, 0)
	m3 := first.AddDate(0, -3, 0)

	e.addAccount(t, e.condoID, "receivable", "condo_fee", secretTitle+" rcv now", 100000, first, "settled", dateStr(first), "")
	e.addAccount(t, e.condoID, "receivable", "condo_fee", secretTitle+" rcv m1", 25050, m1, "settled", dateStr(m1), "")
	e.addAccount(t, e.condoID, "receivable", "fine_interest", secretTitle+" rcv due m3 settled m2", 7000, m3, "settled", dateStr(m2), "")
	e.addAccount(t, e.condoID, "payable", "maintenance", secretTitle+" pay now", 30075, first, "settled", dateStr(first), secretSupplier)
	e.addAccount(t, e.condoID, "payable", "cleaning", secretTitle+" pay m1", 4525, m1, "settled", dateStr(m1), secretSupplier)
	e.addAccount(t, e.condoID, "receivable", "condo_fee", secretTitle+" pending future", 55500, e.now.AddDate(0, 0, 20), "pending", "", "")
	e.addAccount(t, e.condoID, "receivable", "condo_fee", secretTitle+" pending overdue", 12345, e.now.AddDate(0, 0, -40), "pending", "", "")
	e.addAccount(t, e.condoID, "payable", "maintenance", secretTitle+" canceled", 88800, first, "canceled", "", secretSupplier)

	// Another condominium: must never leak into the session condominium's totals.
	e.addAccount(t, e.otherCond, "receivable", "condo_fee", secretTitle+" other rcv", 99999900, first, "settled", dateStr(first), "")
	e.addAccount(t, e.otherCond, "payable", "maintenance", secretTitle+" other pay", 77777700, first, "settled", dateStr(first), "")
	e.addAccount(t, e.otherCond, "receivable", "condo_fee", secretTitle+" other overdue", 66666600, e.now.AddDate(0, 0, -40), "pending", "", "")
	return e
}

const (
	wantIn      = "R$ 1,320.50"
	wantOut     = "R$ 346.00"
	wantBalance = "R$ 974.50"
	wantOverdue = "R$ 123.45"
)

// tile extracts the value rendered under a KPI label ("" when absent).
func tile(body, label string) string {
	re := regexp.MustCompile(`font-medium text-ink-600">` + regexp.QuoteMeta(label) + `</p>\s*<p class="[^"]*">([^<]*)</p>`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return m[1]
}

func totals(body string) [4]string {
	return [4]string{tile(body, "Income"), tile(body, "Expenses"), tile(body, "Period balance"), tile(body, "Overdue")}
}

func assertExpectedTotals(t *testing.T, who, body string) {
	t.Helper()
	want := [4]string{wantIn, wantOut, wantBalance, wantOverdue}
	if got := totals(body); got != want {
		t.Fatalf("%s totals = %v, want %v", who, got, want)
	}
}

func TestFinanceOverviewSameDataForEveryRole(t *testing.T) {
	e := newFinanceEnv(t)
	users := map[string]string{
		"syndic": "syndic@example.com",
		"owner":  "resident@example.com",
		"tenant": "tenant-fin@example.com",
	}
	passwords := map[string]string{
		"syndic": "syndic-pass-123",
		"owner":  "resident-pass-123",
		"tenant": memberPass,
	}
	for role, email := range users {
		c := loginAs(t, e.base, email, passwords[role])
		code, _, body := get(t, c, e.base+"/finance/overview", nil)
		if code != http.StatusOK {
			t.Fatalf("%s overview = %d, want 200", role, code)
		}
		assertExpectedTotals(t, role, body)
	}
}

func TestFinanceOverviewExactValuesAndMonthBuckets(t *testing.T) {
	e := newFinanceEnv(t)
	c := loginAs(t, e.base, "resident@example.com", "resident-pass-123")
	code, _, body := get(t, c, e.base+"/finance/overview", map[string]string{"HX-Request": "true"})
	if code != http.StatusOK {
		t.Fatalf("overview = %d", code)
	}
	assertExpectedTotals(t, "owner", body)

	// The canceled payable (R$ 888.00) counts neither in realized totals nor in
	// "to pay", so its amount must not appear anywhere.
	if strings.Contains(body, "R$ 888.00") {
		t.Errorf("canceled payable amount leaked into the panel")
	}

	first := time.Date(e.now.Year(), e.now.Month(), 1, 0, 0, 0, 0, time.UTC)
	row := func(m time.Time, in, out string) {
		t.Helper()
		label := i18n.T(i18n.LanguageEN, i18n.MessageKey("month.short."+strconv.Itoa(int(m.Month())))) + "/" + m.Format("06")
		re := regexp.MustCompile(`<th scope="row">` + regexp.QuoteMeta(label) + `</th>\s*<td>` + regexp.QuoteMeta(in) +
			`</td>\s*<td>` + regexp.QuoteMeta(out) + `</td>`)
		if !re.MatchString(body) {
			t.Errorf("month %s: no table row with in=%s out=%s", label, in, out)
		}
	}
	row(first, "R$ 1,000.00", "R$ 300.75")
	row(first.AddDate(0, -1, 0), "R$ 250.50", "R$ 45.25")
	// Due in month-3 but settled in month-2: bucketed in the settlement month.
	row(first.AddDate(0, -2, 0), "R$ 70.00", "R$ 0.00")
	row(first.AddDate(0, -3, 0), "R$ 0.00", "R$ 0.00")
}

func TestFinanceOverviewPortugueseFormatting(t *testing.T) {
	e := newFinanceEnv(t)
	if _, err := e.db.Exec(`UPDATE users SET language_preference='pt-br' WHERE email='tenant-fin@example.com'`); err != nil {
		t.Fatalf("set language: %v", err)
	}
	c := loginAs(t, e.base, "tenant-fin@example.com", memberPass)
	_, _, body := get(t, c, e.base+"/finance/overview", nil)
	got := [4]string{tile(body, "Entradas"), tile(body, "Saídas"), tile(body, "Saldo do período"), tile(body, "Em atraso")}
	want := [4]string{"R$ 1.320,50", "R$ 346,00", "R$ 974,50", "R$ 123,45"}
	if got != want {
		t.Fatalf("pt-br totals = %v, want %v", got, want)
	}
	_, _, body = get(t, c, e.base+"/finance/overview?period=2001", nil)
	if !strings.Contains(body, "Nenhuma movimentação financeira em 2001") {
		t.Fatalf("pt-br empty state missing")
	}
}

func TestFinanceOverviewPrivacyAndOtherCondominium(t *testing.T) {
	e := newFinanceEnv(t)
	for _, who := range []struct{ email, pass string }{
		{"syndic@example.com", "syndic-pass-123"},
		{"tenant-fin@example.com", memberPass},
	} {
		c := loginAs(t, e.base, who.email, who.pass)
		for _, hdr := range []map[string]string{nil, {"HX-Request": "true"}} {
			for _, q := range []string{"", "?period=" + strconv.Itoa(e.now.Year())} {
				code, _, body := get(t, c, e.base+"/finance/overview"+q, hdr)
				if code != http.StatusOK {
					t.Fatalf("overview%s = %d", q, code)
				}
				for _, leak := range []string{secretTitle, secretSupplier, "ZZ-SECRET"} {
					if strings.Contains(body, leak) {
						t.Errorf("%s overview%s leaks %q", who.email, q, leak)
					}
				}
				// Other condominium amounts: 999,999.00 / 777,777.00 / 666,666.00.
				for _, other := range []string{"999,999.00", "777,777.00", "666,666.00"} {
					if strings.Contains(body, other) {
						t.Errorf("%s overview%s contains other condominium value %s", who.email, q, other)
					}
				}
				if q == "" {
					assertExpectedTotals(t, who.email, body)
				}
			}
		}
	}
}

func TestFinanceOverviewNoRoleForbidden(t *testing.T) {
	e := newFinanceEnv(t)
	c := loginAs(t, e.base, "norole@example.com", memberPass)

	code, _, body := get(t, c, e.base+"/finance/overview", nil)
	if code != http.StatusForbidden {
		t.Fatalf("no-role overview = %d, want 403", code)
	}
	if strings.Contains(body, "R$") {
		t.Errorf("403 body must not expose values")
	}
	code, _, body = get(t, c, e.base+"/finance/overview", map[string]string{"HX-Request": "true"})
	if code != http.StatusForbidden {
		t.Fatalf("no-role HX overview = %d, want 403", code)
	}
	if strings.Contains(body, "R$") {
		t.Errorf("HX 403 body must not expose values")
	}
}

func TestFinanceOverviewHXWithoutSession(t *testing.T) {
	e := newFinanceEnv(t)
	c := newClient(t)
	code, hdr, _ := get(t, c, e.base+"/finance/overview", map[string]string{"HX-Request": "true"})
	if code >= 300 && code < 400 {
		t.Fatalf("HX without session = %d, must not be 3xx", code)
	}
	if got := hdr.Get("HX-Redirect"); got != "/login" {
		t.Fatalf("HX-Redirect = %q, want /login (status %d)", got, code)
	}
	// Non-HX without a session keeps the standard redirect.
	code, hdr, _ = get(t, c, e.base+"/finance/overview", nil)
	if code != http.StatusSeeOther || !strings.HasPrefix(hdr.Get("Location"), "/login") {
		t.Fatalf("plain without session = %d Location=%q, want 303 to /login", code, hdr.Get("Location"))
	}
}

func TestFinanceOverviewEmptyPeriod(t *testing.T) {
	e := newFinanceEnv(t)
	c := loginAs(t, e.base, "tenant-fin@example.com", memberPass)
	code, _, body := get(t, c, e.base+"/finance/overview?period=2001", map[string]string{"HX-Request": "true"})
	if code != http.StatusOK {
		t.Fatalf("empty period = %d", code)
	}
	if !strings.Contains(body, "No financial activity in 2001") {
		t.Fatalf("empty-state message missing")
	}
	if strings.Contains(body, "data-chart=") {
		t.Errorf("empty period must not render charts")
	}
	// Em atraso is independent of the selected period.
	if got := tile(body, "Overdue"); got != wantOverdue {
		t.Errorf("overdue in empty period = %q, want %q", got, wantOverdue)
	}
}

// With no settlements in the last 12 months the default period shows the
// "last 12 months" empty state, in en and pt-br.
func TestFinanceOverviewEmptyDefaultPeriod(t *testing.T) {
	e := newFinanceEnv(t)
	if _, err := e.db.Exec(`DELETE FROM finance_accounts WHERE condominium_id=$1 AND status='settled'`, e.condoID); err != nil {
		t.Fatalf("clear settlements: %v", err)
	}
	if _, err := e.db.Exec(`UPDATE users SET language_preference='pt-br' WHERE email='tenant-fin@example.com'`); err != nil {
		t.Fatalf("set language: %v", err)
	}
	en := loginAs(t, e.base, "syndic@example.com", "syndic-pass-123")
	_, _, body := get(t, en, e.base+"/finance/overview", nil)
	if !strings.Contains(body, "No financial activity in the last 12 months") {
		t.Fatalf("en default-period empty state missing")
	}
	pt := loginAs(t, e.base, "tenant-fin@example.com", memberPass)
	_, _, body = get(t, pt, e.base+"/finance/overview", nil)
	if !strings.Contains(body, "Nenhuma movimentação financeira nos últimos 12 meses") {
		t.Fatalf("pt-br default-period empty state missing")
	}
}

func TestFinanceOverviewSyndicOnlyLink(t *testing.T) {
	e := newFinanceEnv(t)
	syndic := loginAs(t, e.base, "syndic@example.com", "syndic-pass-123")
	tenant := loginAs(t, e.base, "tenant-fin@example.com", memberPass)
	owner := loginAs(t, e.base, "resident@example.com", "resident-pass-123")

	for _, q := range []string{"", "?period=2001"} {
		_, _, body := get(t, syndic, e.base+"/finance/overview"+q, map[string]string{"HX-Request": "true"})
		if !strings.Contains(body, `href="/finance"`) || !strings.Contains(body, "Manage finances") {
			t.Errorf("syndic overview%s lacks the manage link", q)
		}
		for name, c := range map[string]*http.Client{"tenant": tenant, "owner": owner} {
			_, _, body := get(t, c, e.base+"/finance/overview"+q, map[string]string{"HX-Request": "true"})
			if strings.Contains(body, `href="/finance"`) || strings.Contains(body, "Manage finances") {
				t.Errorf("%s overview%s must not show the manage link", name, q)
			}
			if strings.Contains(body, `href="/finance/new"`) {
				t.Errorf("%s overview%s must not show the register link", name, q)
			}
		}
	}
}

func TestFinanceOverviewFragmentVsPage(t *testing.T) {
	e := newFinanceEnv(t)
	c := loginAs(t, e.base, "tenant-fin@example.com", memberPass)

	code, hdr, frag := get(t, c, e.base+"/finance/overview", map[string]string{"HX-Request": "true"})
	if code != http.StatusOK {
		t.Fatalf("fragment = %d", code)
	}
	if strings.Contains(frag, "<html") {
		t.Errorf("HX response must be a fragment, got a full page")
	}
	if !strings.Contains(frag, `id="finance-overview"`) {
		t.Errorf("fragment lacks section#finance-overview")
	}
	if !strings.Contains(hdr.Get("Vary"), "HX-Request") {
		t.Errorf("Vary = %q, want HX-Request", hdr.Get("Vary"))
	}

	code, _, page := get(t, c, e.base+"/finance/overview", nil)
	if code != http.StatusOK {
		t.Fatalf("page = %d", code)
	}
	if !strings.Contains(page, "<html") || !strings.Contains(page, `id="finance-overview"`) {
		t.Errorf("non-HX response must be a full page containing the panel")
	}
	assertExpectedTotals(t, "page", page)
}

func TestFinanceOverviewInvalidPeriodIsDefault(t *testing.T) {
	e := newFinanceEnv(t)
	c := loginAs(t, e.base, "tenant-fin@example.com", memberPass)
	hx := map[string]string{"HX-Request": "true"}
	_, _, def := get(t, c, e.base+"/finance/overview", hx)
	for _, q := range []string{"?period=abc", "?period=1999", "?period=" + strconv.Itoa(e.now.Year()+1), "?period=20x6", "?period="} {
		code, _, body := get(t, c, e.base+"/finance/overview"+q, hx)
		if code != http.StatusOK {
			t.Fatalf("%s = %d, want 200", q, code)
		}
		if body != def {
			t.Errorf("%s body differs from the default (last 12 months) body", q)
		}
	}
	assertExpectedTotals(t, "default", def)
}

func TestFinanceHealthRedirectsHome(t *testing.T) {
	e := newFinanceEnv(t)
	c := loginAs(t, e.base, "syndic@example.com", "syndic-pass-123")
	code, hdr, _ := get(t, c, e.base+"/finance/health", nil)
	if code != http.StatusSeeOther {
		t.Fatalf("/finance/health = %d, want 303", code)
	}
	if loc := hdr.Get("Location"); loc != "/" {
		t.Fatalf("Location = %q, want /", loc)
	}
}

func TestFinanceHomeShowsPanelPlaceholderForMembersOnly(t *testing.T) {
	e := newFinanceEnv(t)
	for _, who := range []struct{ email, pass string }{
		{"syndic@example.com", "syndic-pass-123"},
		{"resident@example.com", "resident-pass-123"},
		{"tenant-fin@example.com", memberPass},
	} {
		c := loginAs(t, e.base, who.email, who.pass)
		code, body := getPage(t, c, e.base, "/")
		if code != http.StatusOK {
			t.Fatalf("%s home = %d", who.email, code)
		}
		if !strings.Contains(body, `id="finance-overview"`) || !strings.Contains(body, `hx-get="/finance/overview"`) {
			t.Errorf("%s home lacks the financial panel placeholder", who.email)
		}
	}

	// A user whose only role was revoked after login keeps a session condominium
	// but has no active role in it: no panel, and the data is denied.
	insertUserWithRole(t, e.db, "revoked@example.com", "tenant", e.condoID)
	c := loginAs(t, e.base, "revoked@example.com", memberPass)
	if _, err := e.db.Exec(`UPDATE user_roles SET revoked_at = now()
		WHERE user_id = (SELECT id FROM users WHERE email='revoked@example.com')`); err != nil {
		t.Fatalf("revoke role: %v", err)
	}
	code, body := getPage(t, c, e.base, "/")
	if code != http.StatusOK {
		t.Fatalf("revoked-role home = %d, want 200", code)
	}
	if strings.Contains(body, `id="finance-overview"`) || strings.Contains(body, `hx-get="/finance/overview"`) {
		t.Errorf("home of a user without an active role must not contain the financial panel")
	}
	if code, _, _ := get(t, c, e.base+"/finance/overview", nil); code != http.StatusForbidden {
		t.Errorf("revoked-role overview = %d, want 403", code)
	}

	// A user that never had any role has an empty session condominium: the home
	// page must still render (200) without the panel.
	c = loginAs(t, e.base, "norole@example.com", memberPass)
	code, body = getPage(t, c, e.base, "/")
	if code != http.StatusOK {
		t.Fatalf("never-had-a-role home = %d, want 200", code)
	}
	if strings.Contains(body, `id="finance-overview"`) || strings.Contains(body, `hx-get="/finance/overview"`) {
		t.Errorf("never-had-a-role home must not contain the financial panel")
	}
}

func TestFinanceOverviewPerformanceFiveYears(t *testing.T) {
	e := newFinanceEnv(t)
	// ~5 years x 20 settlements per month for the session condominium.
	res, err := e.db.Exec(`
		INSERT INTO finance_accounts
			(condominium_id, account_type, title, category, amount_cents, due_date, status, settlement_date, created_by)
		SELECT $1::uuid,
		       CASE WHEN i <= 10 THEN 'receivable' ELSE 'payable' END,
		       'perf ' || i,
		       CASE WHEN i <= 10 THEN 'condo_fee' ELSE 'maintenance' END,
		       1000 + i,
		       d, 'settled', d, $2::uuid
		FROM generate_series(date_trunc('month', now())::date - interval '59 months',
		                     date_trunc('month', now())::date,
		                     interval '1 month') AS m,
		     generate_series(1, 20) AS i,
		     LATERAL (SELECT (m::date + (i - 1)) AS d) x`, e.condoID, e.syndicID)
	if err != nil {
		t.Fatalf("bulk insert: %v", err)
	}
	if n, _ := res.RowsAffected(); n != 1200 {
		t.Fatalf("bulk inserted %d rows, want 1200", n)
	}

	c := loginAs(t, e.base, "tenant-fin@example.com", memberPass)
	year := strconv.Itoa(e.now.Year() - 2)
	for _, q := range []string{"", "?period=" + year} {
		start := time.Now()
		code, _, body := get(t, c, e.base+"/finance/overview"+q, nil)
		elapsed := time.Since(start)
		if code != http.StatusOK {
			t.Fatalf("overview%s = %d", q, code)
		}
		if !strings.Contains(body, `data-chart="bars"`) {
			t.Errorf("overview%s did not render charts", q)
		}
		if elapsed > 2*time.Second {
			t.Errorf("overview%s took %v, want < 2s", q, elapsed)
		}
		t.Logf("overview%s: %v", q, elapsed)
	}
}
