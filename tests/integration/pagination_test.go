package integration

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// End-to-end pagination tests: every list is 20 rows per page, the nav shows
// "Page N of M" with Previous/Next links that keep the active filters.

const pgSize = 20

var (
	pgTitleRe       = regexp.MustCompile(`<td class="font-medium text-ink-800">([^<]*)`)
	pgTicketTitleRe = regexp.MustCompile(`<a class="link" href="/tickets/[^"]+">([^<]*)</a>`)
	pgCardRe        = regexp.MustCompile(`<p class="text-2xl font-bold text-ink-900">([^<]*)</p>`)
)

// pgPageOf is the nav indicator text.
func pgPageOf(n, total int) string { return fmt.Sprintf(en("pagination.page_of"), n, total) }

// pgBodyRows counts the rows of the first table body found in body.
func pgBodyRows(t *testing.T, body string) int {
	t.Helper()
	return strings.Count(section(t, body, "<tbody>", "</tbody>"), "<tr")
}

// pgTitles returns the text of the first cell of every row (finance, directory,
// suppliers and statement tables), in page order.
func pgTitles(body string) []string {
	var out []string
	for _, m := range pgTitleRe.FindAllStringSubmatch(body, -1) {
		out = append(out, strings.TrimSpace(html.UnescapeString(m[1])))
	}
	return out
}

// pgTicketTitles returns the ticket titles of a tickets table.
func pgTicketTitles(body string) []string {
	var out []string
	for _, m := range pgTicketTitleRe.FindAllStringSubmatch(body, -1) {
		out = append(out, html.UnescapeString(m[1]))
	}
	return out
}

// pgLink returns the parsed target of the nav link with the given label
// ("Next"/"Previous") found in body, or nil when there is none. The href is
// HTML-unescaped (&amp; -> &) first.
func pgLink(t *testing.T, body, label string) *url.URL {
	t.Helper()
	re := regexp.MustCompile(`<a class="btn btn-secondary btn-sm" href="([^"]*)">` + regexp.QuoteMeta(label) + `</a>`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		return nil
	}
	u, err := url.Parse(html.UnescapeString(m[1]))
	if err != nil {
		t.Fatalf("parse %s href %q: %v", label, m[1], err)
	}
	return u
}

// pgNext is pgLink for the "Next" link and fails when it is missing.
func pgNext(t *testing.T, what, body string) *url.URL {
	t.Helper()
	u := pgLink(t, body, en("pagination.next"))
	if u == nil {
		t.Fatalf("%s: no %q link", what, en("pagination.next"))
	}
	return u
}

// pgExpectQuery asserts the link carries exactly the wanted query values.
func pgExpectQuery(t *testing.T, what string, u *url.URL, path string, want url.Values) {
	t.Helper()
	if u.Path != path {
		t.Errorf("%s: path = %q, want %q", what, u.Path, path)
	}
	got := u.Query()
	for k, v := range want {
		if g := got[k]; len(g) != 1 || g[0] != v[0] {
			t.Errorf("%s: %s = %v, want %v (href %s)", what, k, g, v, u.String())
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("%s: unexpected param %q (href %s)", what, k, u.String())
		}
	}
}

// pgGet fetches path as c, requires 200 and returns the body.
func (e *invEnv) pgGet(t *testing.T, c *http.Client, path string) string {
	t.Helper()
	r := e.get(t, c, path)
	if r.code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", path, r.code)
	}
	return r.body
}

// pgDistinct asserts that titles hold want distinct entries.
func pgDistinct(t *testing.T, what string, titles []string, want int) {
	t.Helper()
	seen := map[string]bool{}
	for _, s := range titles {
		if seen[s] {
			t.Errorf("%s: %q listed twice", what, s)
		}
		seen[s] = true
	}
	if len(seen) != want {
		t.Errorf("%s: distinct rows = %d, want %d", what, len(seen), want)
	}
}

func (e *invEnv) pgExec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := e.db.Exec(query, args...); err != nil {
		t.Fatalf("seed %q: %v", query, err)
	}
}

func (e *invEnv) pgUserID(t *testing.T, email string) string {
	t.Helper()
	var id string
	if err := e.db.QueryRow(`SELECT id FROM users WHERE email=$1`, email).Scan(&id); err != nil {
		t.Fatalf("user %s: %v", email, err)
	}
	return id
}

// pgAccounts seeds n accounts titled prefix+NNN. Due dates fall in the current
// month so they count in the dashboard summary.
func (e *invEnv) pgAccounts(t *testing.T, typ, category, prefix, status string, cents int64, n int) {
	t.Helper()
	e.pgExec(t, `
		INSERT INTO finance_accounts (condominium_id, account_type, title, category, amount_cents, due_date, status, settlement_date, created_by)
		SELECT $1::uuid, $2::text, $3::text || lpad(g::text, 3, '0'), $4::text, $5::bigint,
		       date_trunc('month', CURRENT_DATE)::date + (g % 20), $6::text,
		       CASE WHEN $6::text = 'settled' THEN CURRENT_DATE ELSE NULL END, $7::uuid
		FROM generate_series(1, $8::int) g`,
		e.condoA, typ, prefix, category, cents, status, e.syndicAID, n)
}

// AC-1 (FR-1, FR-7, NFR-4): 45 accounts show 20 rows per page over 3 pages.
func TestPaginationFinanceList(t *testing.T) {
	e := newInvEnv(t)
	e.pgAccounts(t, "payable", "utilities", "Account P ", "pending", 10000, 25)
	e.pgAccounts(t, "receivable", "condo_fee", "Account R ", "pending", 20000, 20)

	p1 := e.pgGet(t, e.syndic, "/finance")
	if n := pgBodyRows(t, p1); n != pgSize {
		t.Errorf("page 1 rows = %d, want %d", n, pgSize)
	}
	mustContain(t, "page 1", p1, pgPageOf(1, 3), en("pagination.next"))
	mustNotContain(t, "page 1", p1, ">"+en("pagination.previous")+"<")

	p2 := e.pgGet(t, e.syndic, "/finance?page=2")
	if n := pgBodyRows(t, p2); n != pgSize {
		t.Errorf("page 2 rows = %d, want %d", n, pgSize)
	}
	mustContain(t, "page 2", p2, pgPageOf(2, 3), en("pagination.previous"), en("pagination.next"))

	p3 := e.pgGet(t, e.syndic, "/finance?page=3")
	if n := pgBodyRows(t, p3); n != 5 {
		t.Errorf("page 3 rows = %d, want 5", n)
	}
	mustContain(t, "page 3", p3, pgPageOf(3, 3), en("pagination.previous"))
	mustNotContain(t, "page 3", p3, ">"+en("pagination.next")+"<")

	all := append(append(pgTitles(p1), pgTitles(p2)...), pgTitles(p3)...)
	pgDistinct(t, "finance pages", all, 45)
}

// AC-5 (FR-4): invalid pages render page 1, out of range pages the last page.
func TestPaginationFinanceInvalidPage(t *testing.T) {
	e := newInvEnv(t)
	e.pgAccounts(t, "payable", "utilities", "Account P ", "pending", 10000, 45)

	first := pgTitles(e.pgGet(t, e.syndic, "/finance"))
	if len(first) != pgSize {
		t.Fatalf("page 1 rows = %d, want %d", len(first), pgSize)
	}
	for _, raw := range []string{"abc", "0", "-3", ""} {
		body := e.pgGet(t, e.syndic, "/finance?page="+raw)
		mustContain(t, "page="+raw, body, pgPageOf(1, 3))
		if got := pgTitles(body); strings.Join(got, "|") != strings.Join(first, "|") {
			t.Errorf("page=%q rows differ from page 1", raw)
		}
	}

	body := e.pgGet(t, e.syndic, "/finance?page=999")
	mustContain(t, "page=999", body, pgPageOf(3, 3))
	if n := pgBodyRows(t, body); n != 5 {
		t.Errorf("page=999 rows = %d, want 5", n)
	}
}

// AC-4 (FR-3) and AC-7 (FR-6): nav links keep the filters and drop flash; the
// summary cards do not depend on the page.
func TestPaginationFinanceFiltersAndSummary(t *testing.T) {
	e := newInvEnv(t)
	e.pgAccounts(t, "payable", "utilities", "Conta de agua ", "pending", 10000, 45)
	e.pgAccounts(t, "payable", "utilities", "Conta de agua paga ", "settled", 10000, 5)
	e.pgAccounts(t, "receivable", "condo_fee", "Taxa ", "pending", 20000, 5)

	p1 := e.pgGet(t, e.syndic, "/finance?type=payable&status=pending&q=agua&flash=created&page=1")
	if n := pgBodyRows(t, p1); n != pgSize {
		t.Fatalf("filtered page 1 rows = %d, want %d", n, pgSize)
	}
	mustContain(t, "filtered page 1", p1, pgPageOf(1, 3))
	next := pgNext(t, "filtered page 1", p1)
	pgExpectQuery(t, "next link", next, "/finance", url.Values{
		"type": {"payable"}, "status": {"pending"}, "q": {"agua"}, "page": {"2"},
	})
	if strings.Contains(next.String(), "flash") {
		t.Errorf("next link keeps flash: %s", next)
	}

	// Following the link moves on with the filter intact and links back.
	p2 := e.pgGet(t, e.syndic, next.RequestURI())
	mustContain(t, "filtered page 2", p2, pgPageOf(2, 3))
	prev := pgLink(t, p2, en("pagination.previous"))
	if prev == nil {
		t.Fatal("filtered page 2: no previous link")
	}
	pgExpectQuery(t, "previous link", prev, "/finance", url.Values{
		"type": {"payable"}, "status": {"pending"}, "q": {"agua"}, "page": {"1"},
	})
	p3 := e.pgGet(t, e.syndic, "/finance?type=payable&status=pending&q=agua&page=3")
	if n := pgBodyRows(t, p3); n != 5 {
		t.Errorf("filtered page 3 rows = %d, want 5", n)
	}
	all := append(append(pgTitles(p1), pgTitles(p2)...), pgTitles(p3)...)
	pgDistinct(t, "filtered pages", all, 45)
	for _, title := range all {
		if !strings.HasPrefix(title, "Conta de agua ") || strings.Contains(title, "paga") {
			t.Errorf("row %q does not match the filter", title)
		}
	}

	// AC-7: summary cards equal on every page and cover the whole month, not the page.
	cards := func(body string) []string {
		sec := section(t, body, `<section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">`, `</section>`)
		var out []string
		for _, m := range pgCardRe.FindAllStringSubmatch(sec, -1) {
			out = append(out, m[1])
		}
		if len(out) != 4 {
			t.Fatalf("summary cards = %v, want 4 values", out)
		}
		return out
	}
	s1 := e.pgGet(t, e.syndic, "/finance")
	s2 := e.pgGet(t, e.syndic, "/finance?page=2")
	s3 := e.pgGet(t, e.syndic, "/finance?page=3")
	c1, c2, c3 := cards(s1), cards(s2), cards(s3)
	for i := range c1 {
		if c1[i] != c2[i] || c1[i] != c3[i] {
			t.Errorf("summary card %d differs across pages: %q %q %q", i, c1[i], c2[i], c3[i])
		}
	}
	// Payables 45 pending + 5 settled = 50 x 100.00; receivables 5 x 200.00.
	if c1[0] != fmtMoney(100000) || c1[1] != fmtMoney(500000) {
		t.Errorf("summary = receivable %q payable %q, want %q and %q", c1[0], c1[1], fmtMoney(100000), fmtMoney(500000))
	}
	// A filtered page shows the same cards as the unfiltered one (FR-6).
	if cf := cards(p1); strings.Join(cf, "|") != strings.Join(c1, "|") {
		t.Errorf("filtered summary %v differs from unfiltered %v", cf, c1)
	}
}

// AC-8 (FR-1, FR-7): tickets mine and the syndic inbox.
func TestPaginationTickets(t *testing.T) {
	e := newInvEnv(t)
	resident := e.pgUserID(t, "resident@example.com")
	seed := func(author, prefix, status string, n int) {
		e.pgExec(t, `
			INSERT INTO tickets (condominium_id, author_id, unit_id, category, title, description, status, closed_at, closed_by, created_at)
			SELECT $1::uuid, $2::uuid, NULL, 'repair', $3::text || lpad(g::text, 3, '0'), 'desc', $4::text,
			       CASE WHEN $4::text = 'closed' THEN now() ELSE NULL END,
			       CASE WHEN $4::text = 'closed' THEN $5::uuid ELSE NULL END,
			       now() - g * interval '1 minute'
			FROM generate_series(1, $6::int) g`,
			e.condoA, author, prefix, status, e.syndicAID, n)
	}
	seed(resident, "Mine ", "open", 45)
	// Tickets of others never show on the resident's list.
	seed(e.syndicAID, "Other ", "open", 3)

	// Mine: 45 tickets, 3 pages.
	m1 := e.pgGet(t, e.owner, "/tickets")
	if n := pgBodyRows(t, m1); n != pgSize {
		t.Errorf("mine page 1 rows = %d, want %d", n, pgSize)
	}
	mustContain(t, "mine page 1", m1, pgPageOf(1, 3))
	pgExpectQuery(t, "mine next", pgNext(t, "mine page 1", m1), "/tickets", url.Values{"page": {"2"}})
	m2 := e.pgGet(t, e.owner, "/tickets?page=2")
	m3 := e.pgGet(t, e.owner, "/tickets?page=3")
	if n := pgBodyRows(t, m3); n != 5 {
		t.Errorf("mine page 3 rows = %d, want 5", n)
	}
	mustContain(t, "mine page 3", m3, pgPageOf(3, 3), en("pagination.previous"))
	mine := append(append(pgTicketTitles(m1), pgTicketTitles(m2)...), pgTicketTitles(m3)...)
	pgDistinct(t, "mine pages", mine, 45)
	for _, title := range mine {
		if !strings.HasPrefix(title, "Mine ") {
			t.Errorf("foreign ticket %q on the resident list", title)
		}
	}

	// Inbox: 30 open and 25 closed tickets (status defaults to open).
	e.pgExec(t, `DELETE FROM tickets`)
	seed(resident, "Open ", "open", 30)
	seed(resident, "Closed ", "closed", 25)

	i1 := e.pgGet(t, e.syndic, "/tickets/inbox")
	if n := pgBodyRows(t, i1); n != pgSize {
		t.Errorf("inbox page 1 rows = %d, want %d", n, pgSize)
	}
	mustContain(t, "inbox page 1", i1, pgPageOf(1, 2))
	i2 := e.pgGet(t, e.syndic, "/tickets/inbox?page=2")
	if n := pgBodyRows(t, i2); n != 10 {
		t.Errorf("inbox page 2 rows = %d, want 10", n)
	}
	pgDistinct(t, "inbox open pages", append(pgTicketTitles(i1), pgTicketTitles(i2)...), 30)

	c1 := e.pgGet(t, e.syndic, "/tickets/inbox?status=closed")
	mustContain(t, "closed page 1", c1, pgPageOf(1, 2))
	pgExpectQuery(t, "closed next", pgNext(t, "closed page 1", c1), "/tickets/inbox",
		url.Values{"status": {"closed"}, "page": {"2"}})
	c2 := e.pgGet(t, e.syndic, "/tickets/inbox?status=closed&page=2")
	if n := pgBodyRows(t, c2); n != 5 {
		t.Errorf("closed page 2 rows = %d, want 5", n)
	}
	pgDistinct(t, "inbox closed pages", append(pgTicketTitles(c1), pgTicketTitles(c2)...), 25)

	a1 := e.pgGet(t, e.syndic, "/tickets/inbox?status=all")
	mustContain(t, "all page 1", a1, pgPageOf(1, 3))
	pgExpectQuery(t, "all next", pgNext(t, "all page 1", a1), "/tickets/inbox",
		url.Values{"status": {"all"}, "page": {"2"}})
}

// AC-8 (FR-1, FR-7): directory listings keep q and category on the nav links.
func TestPaginationDirectory(t *testing.T) {
	e := newInvEnv(t)
	var plumbing, electric string
	if err := e.db.QueryRow(`INSERT INTO service_categories (condominium_id, name) VALUES ($1, 'Plumbing') RETURNING id`, e.condoA).Scan(&plumbing); err != nil {
		t.Fatalf("category: %v", err)
	}
	if err := e.db.QueryRow(`INSERT INTO service_categories (condominium_id, name) VALUES ($1, 'Electric') RETURNING id`, e.condoA).Scan(&electric); err != nil {
		t.Fatalf("category: %v", err)
	}
	seed := func(category, prefix string, n int) {
		e.pgExec(t, `
			INSERT INTO service_provider_listings
				(condominium_id, category_id, name, phone, phone_digits, notes, recommended_by_unit_id, recommended_by_unit_code, submitted_by)
			SELECT $1::uuid, $2::uuid, $3::text || lpad(g::text, 3, '0'), '+55 11 99999-0000', '5511999990000', NULL,
			       NULL, 'B-1', $4::uuid
			FROM generate_series(1, $5::int) g`,
			e.condoA, category, prefix, e.syndicAID, n)
	}
	seed(plumbing, "Plumber ", 45)
	seed(electric, "Sparky ", 25)

	p1 := e.pgGet(t, e.syndic, "/directory")
	if n := pgBodyRows(t, p1); n != pgSize {
		t.Errorf("page 1 rows = %d, want %d", n, pgSize)
	}
	mustContain(t, "page 1", p1, pgPageOf(1, 4))
	p4 := e.pgGet(t, e.syndic, "/directory?page=4")
	if n := pgBodyRows(t, p4); n != 10 {
		t.Errorf("page 4 rows = %d, want 10", n)
	}
	var all []string
	for p := 1; p <= 4; p++ {
		all = append(all, pgTitles(e.pgGet(t, e.syndic, fmt.Sprintf("/directory?page=%d", p)))...)
	}
	pgDistinct(t, "directory pages", all, 70)

	// q and category are kept; ILIKE name match narrows to the plumbers.
	f1 := e.pgGet(t, e.syndic, "/directory?q=plumber&category="+plumbing)
	mustContain(t, "filtered page 1", f1, pgPageOf(1, 3))
	next := pgNext(t, "filtered page 1", f1)
	pgExpectQuery(t, "next link", next, "/directory", url.Values{
		"q": {"plumber"}, "category": {plumbing}, "page": {"2"},
	})
	f2 := e.pgGet(t, e.syndic, next.RequestURI())
	f3 := e.pgGet(t, e.syndic, "/directory?q=plumber&category="+plumbing+"&page=3")
	if n := pgBodyRows(t, f3); n != 5 {
		t.Errorf("filtered page 3 rows = %d, want 5", n)
	}
	filtered := append(append(pgTitles(f1), pgTitles(f2)...), pgTitles(f3)...)
	pgDistinct(t, "filtered directory pages", filtered, 45)
	for _, name := range filtered {
		if !strings.HasPrefix(name, "Plumber ") {
			t.Errorf("row %q does not match the filter", name)
		}
	}

	// Category alone: 25 electricians over 2 pages.
	c1 := e.pgGet(t, e.syndic, "/directory?category="+electric)
	mustContain(t, "category page 1", c1, pgPageOf(1, 2))
	pgExpectQuery(t, "category next", pgNext(t, "category page 1", c1), "/directory",
		url.Values{"category": {electric}, "page": {"2"}})
}

// AC-8 (FR-1, FR-7) and AC-12 (FR-11): the suppliers list reaches all 60
// suppliers over 3 pages and keeps q; the invoice form autocomplete is capped.
func TestPaginationSuppliers(t *testing.T) {
	e := newInvEnv(t)
	e.pgExec(t, `
		INSERT INTO suppliers (condominium_id, name, nif, created_by, updated_by)
		SELECT $1::uuid, 'Supplier ' || lpad(g::text, 3, '0'), '', $2::uuid, $2::uuid
		FROM generate_series(1, 60) g`, e.condoA, e.syndicAID)

	var all []string
	for p := 1; p <= 3; p++ {
		body := e.pgGet(t, e.syndic, fmt.Sprintf("/suppliers?page=%d", p))
		if n := pgBodyRows(t, body); n != pgSize {
			t.Errorf("suppliers page %d rows = %d, want %d", p, n, pgSize)
		}
		mustContain(t, fmt.Sprintf("suppliers page %d", p), body, pgPageOf(p, 3))
		all = append(all, pgTitles(body)...)
	}
	pgDistinct(t, "suppliers pages", all, 60)
	sorted := append([]string(nil), all...)
	sort.Strings(sorted)
	if strings.Join(sorted, "|") != strings.Join(all, "|") {
		t.Errorf("suppliers are not in name order across pages")
	}

	// q is kept on the links and the filtered list pages the same way.
	f1 := e.pgGet(t, e.syndic, "/suppliers?q=supplier")
	mustContain(t, "filtered page 1", f1, pgPageOf(1, 3))
	pgExpectQuery(t, "next link", pgNext(t, "filtered page 1", f1), "/suppliers",
		url.Values{"q": {"supplier"}, "page": {"2"}})
	f3 := e.pgGet(t, e.syndic, "/suppliers?q=supplier&page=3")
	prev := pgLink(t, f3, en("pagination.previous"))
	if prev == nil {
		t.Fatal("filtered page 3: no previous link")
	}
	pgExpectQuery(t, "previous link", prev, "/suppliers", url.Values{"q": {"supplier"}, "page": {"2"}})

	// A narrower q lists fewer rows and needs no nav.
	one := e.pgGet(t, e.syndic, "/suppliers?q=Supplier+007")
	if n := pgBodyRows(t, one); n != 1 {
		t.Errorf("q=Supplier 007 rows = %d, want 1", n)
	}
	mustNotContain(t, "single page", one, `aria-label="`+en("pagination.label")+`"`)

	// AC-12: the autocomplete is not paged and keeps its limit (10 in
	// supplierSearchGET) even though 60 suppliers match.
	r := e.get(t, e.syndic, "/suppliers/search?q=Supplier", hx)
	if r.code != http.StatusOK {
		t.Fatalf("autocomplete = %d, want 200", r.code)
	}
	if n := strings.Count(r.body, "<li>"); n != 10 {
		t.Errorf("autocomplete results = %d, want 10", n)
	}
	mustNotContain(t, "autocomplete", r.body, en("pagination.next"), "Page ")
}

// AC-11 (FR-10) and AC-7 (FR-6): the invoice list shows 20 rows per page and
// the totals cover the whole filtered set on every page.
func TestPaginationInvoices(t *testing.T) {
	e := newInvEnv(t)
	sup := e.newSupplier(t, e.syndic, e.condoA, "Many Lda", validNIF("50000013"))
	e.pgExec(t, `
		INSERT INTO invoices (condominium_id, state, document_type, expense_type, number, issue_date,
			supplier_id, category, description, total_cents, payable_cents, created_by, updated_by)
		SELECT $1::uuid, 'registered', 'fatura', 'manutencao', 'P-' || lpad(g::text, 3, '0'), DATE '2026-01-01' + g,
			$2::uuid, 'manutencao', 'bulk', 10000, 10000, $3::uuid, $3::uuid
		FROM generate_series(1, 45) g`, e.condoA, sup, e.syndicAID)

	var numbers []string
	for p, want := range map[int]int{1: pgSize, 2: pgSize, 3: 5} {
		body := e.pgGet(t, e.syndic, fmt.Sprintf("/invoices?page=%d", p))
		if n := len(tableRows(body)); n != want {
			t.Errorf("invoices page %d rows = %d, want %d", p, n, want)
		}
		mustContain(t, fmt.Sprintf("invoices page %d", p), body, pgPageOf(p, 3))
		mustContain(t, fmt.Sprintf("invoices page %d totals", p),
			section(t, body, `data-totals-footer`, `</section>`), fmtMoney(450000))
		for _, row := range tableRows(body) {
			m := regexp.MustCompile(`>(P-\d+)</a>`).FindStringSubmatch(row)
			if m == nil {
				t.Fatalf("invoice row without number: %s", row)
			}
			numbers = append(numbers, m[1])
		}
	}
	pgDistinct(t, "invoice pages", numbers, 45)

	body := e.pgGet(t, e.syndic, "/invoices")
	pgExpectQuery(t, "invoices next", pgNext(t, "invoices page 1", body), "/invoices", url.Values{"page": {"2"}})
	// Out of range resolves to the last page.
	last := e.pgGet(t, e.syndic, "/invoices?page=999")
	mustContain(t, "page 999", last, pgPageOf(3, 3))
}

// AC-10 (FR-9): on the unit statement each table pages on its own parameter and
// following one nav link leaves the others alone.
func TestPaginationUnitStatement(t *testing.T) {
	e := newQuotaEnv(t)
	unit := e.addUnit(t, "S-1", 1000)
	seed := func(prefix, status string, n int) {
		e.pgExec(t, `
			INSERT INTO finance_accounts (condominium_id, account_type, title, category, amount_cents, due_date, status, settlement_date, unit_id, created_by)
			SELECT $1::uuid, 'receivable', $2::text || lpad(g::text, 3, '0'), 'condo_fee', 10000,
			       CURRENT_DATE + 30 + g, $3::text,
			       CASE WHEN $3::text = 'settled' THEN CURRENT_DATE - g ELSE NULL END, $4::uuid, $5::uuid
			FROM generate_series(1, $6::int) g`,
			e.condoA, prefix, status, unit, e.syndicAID, n)
	}
	seed("Pend ", "pending", 45)
	seed("Paid ", "settled", 45)
	seed("Void ", "canceled", 25)

	titles := func(body, key string) []string {
		sec := section(t, body, `<h2 class="text-lg font-bold text-ink-900">`+en(key)+`</h2>`, `</section>`)
		return pgTitles(sec)
	}
	secOf := func(body, key string) string {
		return section(t, body, `<h2 class="text-lg font-bold text-ink-900">`+en(key)+`</h2>`, `</section>`)
	}

	start := "/finance/my-charges?unit=" + unit + "&pending_page=2&canceled_page=2"
	base := e.pgGet(t, e.syndic, start)
	basePending := titles(base, "quotas.statement.pending")
	baseSettled := titles(base, "quotas.statement.settled")
	baseCanceled := titles(base, "quotas.statement.canceled")
	if len(basePending) != pgSize || len(baseSettled) != pgSize || len(baseCanceled) != 5 {
		t.Fatalf("rows pending/settled/canceled = %d/%d/%d, want 20/20/5", len(basePending), len(baseSettled), len(baseCanceled))
	}
	mustContain(t, "pending nav", secOf(base, "quotas.statement.pending"), pgPageOf(2, 3))
	mustContain(t, "settled nav", secOf(base, "quotas.statement.settled"), pgPageOf(1, 3))
	mustContain(t, "canceled nav", secOf(base, "quotas.statement.canceled"), pgPageOf(2, 2))

	// Every nav link carries the unit and its own page key.
	pgExpectQuery(t, "pending next", pgNext(t, "pending", secOf(base, "quotas.statement.pending")),
		"/finance/my-charges", url.Values{"unit": {unit}, "pending_page": {"3"}, "canceled_page": {"2"}})
	prev := pgLink(t, secOf(base, "quotas.statement.canceled"), en("pagination.previous"))
	if prev == nil {
		t.Fatal("canceled: no previous link")
	}
	pgExpectQuery(t, "canceled previous", prev, "/finance/my-charges",
		url.Values{"unit": {unit}, "pending_page": {"2"}, "canceled_page": {"1"}})

	// Follow "Next" of the settled table: only settled_page changes.
	next := pgNext(t, "settled", secOf(base, "quotas.statement.settled"))
	pgExpectQuery(t, "settled next", next, "/finance/my-charges", url.Values{
		"unit": {unit}, "pending_page": {"2"}, "canceled_page": {"2"}, "settled_page": {"2"},
	})
	moved := e.pgGet(t, e.syndic, next.RequestURI())
	if got := titles(moved, "quotas.statement.pending"); strings.Join(got, "|") != strings.Join(basePending, "|") {
		t.Errorf("pending rows changed when paging the settled table")
	}
	if got := titles(moved, "quotas.statement.canceled"); strings.Join(got, "|") != strings.Join(baseCanceled, "|") {
		t.Errorf("canceled rows changed when paging the settled table")
	}
	movedSettled := titles(moved, "quotas.statement.settled")
	if len(movedSettled) != pgSize {
		t.Errorf("settled page 2 rows = %d, want %d", len(movedSettled), pgSize)
	}
	mustContain(t, "settled page 2", secOf(moved, "quotas.statement.settled"), pgPageOf(2, 3))
	pgDistinct(t, "settled pages 1+2", append(append([]string(nil), baseSettled...), movedSettled...), 2*pgSize)

	// The summary cards (all pending of the unit) are the same on both pages.
	if a, b := pgCardRe.FindAllString(base, -1), pgCardRe.FindAllString(moved, -1); strings.Join(a, "|") != strings.Join(b, "|") {
		t.Errorf("statement summary changed between pages: %v vs %v", a, b)
	}
}
