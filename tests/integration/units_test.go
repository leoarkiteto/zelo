package integration

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// AC-1 and the unit part of AC-21: the syndic creates unit C with 250 per
// mille, edits A to 400 and the list shows both values plus the sum and the
// difference to 100%.
func TestUnitsCreateEditAndSum(t *testing.T) {
	e := newQuotaEnv(t)
	aID := e.addUnit(t, "A", 100000)

	// The form pages carry the CSRF token the posts below use.
	code, body := getPage(t, e.syndic, e.base, "/condominium/units/new")
	if code != http.StatusOK || !csrfRe.MatchString(body) {
		t.Fatalf("new unit form = %d, csrf present = %v", code, csrfRe.MatchString(body))
	}
	r := e.post(t, e.syndic, "/condominium/units", url.Values{"code": {"C"}, "share": {"250"}})
	if r.code != http.StatusSeeOther || !strings.Contains(r.location(), "/condominium/units") {
		t.Fatalf("create unit = %d Location=%q, want 303 to the list\n%s", r.code, r.location(), r.body)
	}
	r = e.post(t, e.syndic, "/condominium/units/"+aID+"/edit", url.Values{"code": {"A"}, "share": {"400"}})
	if r.code != http.StatusSeeOther {
		t.Fatalf("edit unit = %d, want 303\n%s", r.code, r.body)
	}

	var cPPM, aPPM int
	if err := e.db.QueryRow(`SELECT share_ppm FROM units WHERE condominium_id=$1 AND code='C'`, e.condoA).Scan(&cPPM); err != nil {
		t.Fatalf("read C: %v", err)
	}
	if err := e.db.QueryRow(`SELECT share_ppm FROM units WHERE id=$1`, aID).Scan(&aPPM); err != nil {
		t.Fatalf("read A: %v", err)
	}
	if cPPM != 250000 || aPPM != 400000 {
		t.Fatalf("stored shares C=%d A=%d, want 250000 and 400000", cPPM, aPPM)
	}

	// The seeded unit B-1 has a share of 0: the sum is 650 and 350 are missing.
	r = e.get(t, e.syndic, "/condominium/units")
	if r.code != http.StatusOK {
		t.Fatalf("units list = %d", r.code)
	}
	mustContain(t, "units list", r.body,
		"B-1", ">A<", ">C<", ">400<", ">250<",
		`data-testid="share-sum">650‰`, `data-testid="share-diff">350‰`,
		"The shares are below 100%.")

	if n := e.auditCount(t, "unit_created", e.syndicAID, ""); n != 1 {
		t.Errorf("unit_created events = %d, want 1", n)
	}
	if n := e.auditCount(t, "unit_updated", e.syndicAID, ""); n != 1 {
		t.Errorf("unit_updated events = %d, want 1", n)
	}
}

// AC-2: a duplicate code, a negative share, a share above 1000 and a share with
// more than three decimals are refused with a message and nothing is stored.
func TestUnitsRejectInvalidInput(t *testing.T) {
	e := newQuotaEnv(t)
	aID := e.addUnit(t, "A", 100000)
	before := e.count(t, `SELECT count(*) FROM units WHERE condominium_id=$1`, e.condoA)

	r := e.post(t, e.syndic, "/condominium/units", url.Values{"code": {"A"}, "share": {"10"}})
	if r.code != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate code = %d, want 422", r.code)
	}
	mustContain(t, "duplicate code", r.body, "Another unit already uses this code.")

	for _, share := range []string{"-1", "1000.5", "1001", "10.1234", "abc", ""} {
		r := e.post(t, e.syndic, "/condominium/units", url.Values{"code": {"Z9"}, "share": {share}})
		if r.code != http.StatusUnprocessableEntity {
			t.Errorf("share %q = %d, want 422", share, r.code)
			continue
		}
		mustContain(t, "share "+share, r.body, "Enter a share from 0 to 1000 with up to three decimals.")
	}
	if got := e.count(t, `SELECT count(*) FROM units WHERE condominium_id=$1`, e.condoA); got != before {
		t.Fatalf("units after refused creates = %d, want %d", got, before)
	}

	// Editing: a code taken by another unit and an invalid share change nothing.
	e.addUnit(t, "B", 0)
	r = e.post(t, e.syndic, "/condominium/units/"+aID+"/edit", url.Values{"code": {"B"}, "share": {"100"}})
	if r.code != http.StatusUnprocessableEntity {
		t.Fatalf("edit to duplicate code = %d, want 422", r.code)
	}
	r = e.post(t, e.syndic, "/condominium/units/"+aID+"/edit", url.Values{"code": {"A"}, "share": {"1000.001"}})
	if r.code != http.StatusUnprocessableEntity {
		t.Fatalf("edit with share > 1000 = %d, want 422", r.code)
	}
	var code string
	var ppm int
	if err := e.db.QueryRow(`SELECT code, share_ppm FROM units WHERE id=$1`, aID).Scan(&code, &ppm); err != nil {
		t.Fatalf("read A: %v", err)
	}
	if code != "A" || ppm != 100000 {
		t.Fatalf("unit A after refused edits = %s/%d, want A/100000", code, ppm)
	}
	if n := e.auditCount(t, "unit_created", "", ""); n != 0 {
		t.Errorf("refused creates logged %d unit_created events, want 0", n)
	}
}

// AC-23 (units part): owners and tenants receive 403 on every unit screen and
// action, and nothing changes.
func TestUnitsForbiddenForNonSyndic(t *testing.T) {
	e := newQuotaEnv(t)
	aID := e.addUnit(t, "A", 100000)
	for name, c := range map[string]*http.Client{"owner": e.owner, "tenant": e.tenant} {
		for _, path := range []string{"/condominium/units", "/condominium/units/new", "/condominium/units/" + aID + "/edit"} {
			if r := e.get(t, c, path); r.code != http.StatusForbidden {
				t.Errorf("%s GET %s = %d, want 403", name, path, r.code)
			}
		}
		if r := e.post(t, c, "/condominium/units", url.Values{"code": {"HACK"}, "share": {"1"}}); r.code != http.StatusForbidden {
			t.Errorf("%s POST create unit = %d, want 403", name, r.code)
		}
		if r := e.post(t, c, "/condominium/units/"+aID+"/edit", url.Values{"code": {"A"}, "share": {"999"}}); r.code != http.StatusForbidden {
			t.Errorf("%s POST edit unit = %d, want 403", name, r.code)
		}
	}
	var ppm int
	if err := e.db.QueryRow(`SELECT share_ppm FROM units WHERE id=$1`, aID).Scan(&ppm); err != nil || ppm != 100000 {
		t.Fatalf("unit A share = %d (%v), want 100000", ppm, err)
	}
	if n := e.count(t, `SELECT count(*) FROM units WHERE code='HACK'`); n != 0 {
		t.Fatalf("forbidden create stored %d units", n)
	}
}
