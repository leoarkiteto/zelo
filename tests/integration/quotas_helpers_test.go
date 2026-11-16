package integration

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// quotaEnv extends invEnv (condominium A with its syndic, the owner "resident"
// who occupies unit B-1, and a tenant without occupancy; condominium B with its
// own syndic) with fixtures for units, budgets and issuances.
type quotaEnv struct {
	*invEnv
}

func newQuotaEnv(t *testing.T) *quotaEnv {
	t.Helper()
	return &quotaEnv{invEnv: newInvEnv(t)}
}

var budgetLocRe = regexp.MustCompile(`^/finance/budgets/([0-9a-f-]{36})/view`)

// addUnit inserts a unit of condominium A with the given share (ppm).
func (e *quotaEnv) addUnit(t *testing.T, code string, ppm int) string {
	t.Helper()
	var id string
	if err := e.db.QueryRow(
		`INSERT INTO units (condominium_id, code, share_ppm) VALUES ($1, $2, $3) RETURNING id`,
		e.condoA, code, ppm).Scan(&id); err != nil {
		t.Fatalf("insert unit %s: %v", code, err)
	}
	return id
}

// addStdUnits creates A (400), B (350) and C (250) per mille: 100% with the
// seeded zero-share unit B-1, which never receives a quota.
func (e *quotaEnv) addStdUnits(t *testing.T) (a, b, c string) {
	t.Helper()
	return e.addUnit(t, "A", 400000), e.addUnit(t, "B", 350000), e.addUnit(t, "C", 250000)
}

// occupy gives the user an active occupancy of the unit starting at startedAt
// (a date, "2026-10-01").
func (e *quotaEnv) occupy(t *testing.T, email, unitID, occType, startedAt string) {
	t.Helper()
	if _, err := e.db.Exec(`
		INSERT INTO unit_occupancies (user_id, unit_id, occupancy_type, started_at)
		SELECT id, $2::uuid, $3::occupancy_type, $4::timestamptz FROM users WHERE email=$1`,
		email, unitID, occType, startedAt+"T00:00:00Z"); err != nil {
		t.Fatalf("occupy %s: %v", email, err)
	}
}

func (e *quotaEnv) userID(t *testing.T, email string) string {
	t.Helper()
	var id string
	if err := e.db.QueryRow(`SELECT id FROM users WHERE email=$1`, email).Scan(&id); err != nil {
		t.Fatalf("user %s: %v", email, err)
	}
	return id
}

// createBudget posts the create form as the syndic of A and returns the id.
func (e *quotaEnv) createBudget(t *testing.T, year int, periodicity string, dueDay int, rate string) string {
	t.Helper()
	r := e.post(t, e.syndic, "/finance/budgets", url.Values{
		"year": {strconv.Itoa(year)}, "periodicity": {periodicity},
		"due_day": {strconv.Itoa(dueDay)}, "reserve_rate": {rate},
	})
	if r.code != http.StatusSeeOther {
		t.Fatalf("create budget %d = %d, want 303\n%s", year, r.code, r.body)
	}
	m := budgetLocRe.FindStringSubmatch(r.location())
	if m == nil {
		t.Fatalf("create budget: unexpected Location %q", r.location())
	}
	return m[1]
}

func (e *quotaEnv) addLine(t *testing.T, budgetID, desc, category, amount string) resp {
	t.Helper()
	return e.post(t, e.syndic, "/finance/budgets/"+budgetID+"/lines", url.Values{
		"description": {desc}, "category": {category}, "amount": {amount},
	})
}

func (e *quotaEnv) approve(t *testing.T, budgetID string) resp {
	t.Helper()
	return e.post(t, e.syndic, "/finance/budgets/"+budgetID+"/approve", nil)
}

// newApprovedBudget creates, fills (8000.00 + 4000.00 = 12000.00) and approves
// a budget. With a 10% reserve every monthly period base is 1000.00.
func (e *quotaEnv) newApprovedBudget(t *testing.T, year int, periodicity string, dueDay int, rate string) string {
	t.Helper()
	id := e.createBudget(t, year, periodicity, dueDay, rate)
	if r := e.addLine(t, id, "Maintenance", "maintenance", "8000.00"); r.code != http.StatusSeeOther {
		t.Fatalf("add line = %d\n%s", r.code, r.body)
	}
	if r := e.addLine(t, id, "Cleaning", "cleaning", "4000.00"); r.code != http.StatusSeeOther {
		t.Fatalf("add line = %d\n%s", r.code, r.body)
	}
	if r := e.approve(t, id); r.code != http.StatusSeeOther {
		t.Fatalf("approve = %d\n%s", r.code, r.body)
	}
	return id
}

func (e *quotaEnv) issue(t *testing.T, budgetID string, period int) resp {
	t.Helper()
	return e.post(t, e.syndic, "/finance/budgets/"+budgetID+"/issuances/"+strconv.Itoa(period), nil)
}

// activeIssuanceID returns the active issuance of a budget period ("" if none).
func (e *quotaEnv) activeIssuanceID(t *testing.T, budgetID string, period int) string {
	t.Helper()
	var id string
	err := e.db.QueryRow(`SELECT id FROM quota_issuances WHERE budget_id=$1 AND period_index=$2 AND status='issued'`,
		budgetID, period).Scan(&id)
	if err != nil {
		return ""
	}
	return id
}

// quotaRow is one quota receivable joined with its unit and detail.
type quotaRow struct {
	AccountID string
	Unit      string
	Cents     int64
	Due       string
	Status    string
	Category  string
	Type      string
	SharePPM  int
	Base      int64
	Reserve   int64
}

func (e *quotaEnv) quotaRows(t *testing.T, issuanceID string) []quotaRow {
	t.Helper()
	rows, err := e.db.Query(`
		SELECT a.id, u.code, a.amount_cents, to_char(a.due_date, 'YYYY-MM-DD'), a.status, a.category, a.account_type,
		       c.share_ppm, c.base_cents, c.reserve_cents
		FROM quota_charges c
		JOIN finance_accounts a ON a.id = c.finance_account_id
		JOIN units u ON u.id = c.unit_id
		WHERE c.issuance_id = $1 ORDER BY u.code`, issuanceID)
	if err != nil {
		t.Fatalf("quota rows: %v", err)
	}
	defer rows.Close()
	var out []quotaRow
	for rows.Next() {
		var r quotaRow
		if err := rows.Scan(&r.AccountID, &r.Unit, &r.Cents, &r.Due, &r.Status, &r.Category, &r.Type, &r.SharePPM, &r.Base, &r.Reserve); err != nil {
			t.Fatalf("scan quota row: %v", err)
		}
		out = append(out, r)
	}
	return out
}

// quotaAccountID returns the receivable id of a unit's quota in an issuance.
func (e *quotaEnv) quotaAccountID(t *testing.T, issuanceID, unitCode string) string {
	t.Helper()
	for _, r := range e.quotaRows(t, issuanceID) {
		if r.Unit == unitCode {
			return r.AccountID
		}
	}
	t.Fatalf("no quota for unit %s in issuance %s", unitCode, issuanceID)
	return ""
}

// insertReceivable inserts a plain receivable of a unit (unitID may be "").
// settled == "" means no settlement date.
func (e *quotaEnv) insertReceivable(t *testing.T, unitID, title string, cents int64, due, status, settled string) string {
	t.Helper()
	var unitArg, settledArg any
	if unitID != "" {
		unitArg = unitID
	}
	if settled != "" {
		settledArg = settled
	}
	var id string
	if err := e.db.QueryRow(`
		INSERT INTO finance_accounts
			(condominium_id, account_type, title, category, amount_cents, due_date, status, settlement_date, unit_id, created_by)
		VALUES ($1, 'receivable', $2, 'condo_fee', $3, $4, $5, $6, $7, $8) RETURNING id`,
		e.condoA, title, cents, due, status, settledArg, unitArg, e.syndicAID).Scan(&id); err != nil {
		t.Fatalf("insert receivable %s: %v", title, err)
	}
	return id
}

// auditCount counts audit events of a type, optionally filtered by user and by
// details->>'resource' (empty strings match anything).
func (e *quotaEnv) auditCount(t *testing.T, event, userID, resource string) int {
	t.Helper()
	return e.count(t, `SELECT count(*) FROM audit_events
		WHERE event_type=$1 AND ($2 = '' OR user_id::text=$2) AND ($3 = '' OR details->>'resource'=$3)`,
		event, userID, resource)
}

// postRaw is a goroutine-safe POST: it never calls t.Fatal. The CSRF token must
// be resolved beforehand (e.csrf) on the test goroutine.
func (e *quotaEnv) postRaw(c *http.Client, token, path string, v url.Values) (int, string, error) {
	if v == nil {
		v = url.Values{}
	}
	v.Set("csrf_token", token)
	req, err := http.NewRequest(http.MethodPost, e.base+path, strings.NewReader(v.Encode()))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	loc := res.Header.Get("Location")
	return res.StatusCode, loc + "\n" + string(b), nil
}

// inputValue extracts the value attribute of the <input> with the given name.
func inputValue(body, name string) (string, bool) {
	re := regexp.MustCompile(`<input[^>]*name="` + regexp.QuoteMeta(name) + `"[^>]*>`)
	tag := re.FindString(body)
	if tag == "" {
		return "", false
	}
	m := regexp.MustCompile(`value="([^"]*)"`).FindStringSubmatch(tag)
	if m == nil {
		return "", true
	}
	return m[1], true
}
