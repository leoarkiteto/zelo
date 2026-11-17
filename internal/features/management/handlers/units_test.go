package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/features/management/services"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

type fakeRoles struct{ roles []model.Role }

func (f *fakeRoles) ActiveRolesForUser(context.Context, string, string) ([]model.Role, error) {
	return f.roles, nil
}

func (f *fakeRoles) ListUsersWithRoles(context.Context, string) ([]store.UserRolesRow, error) {
	return nil, nil
}

type fakeSessionReader struct{ sess model.Session }

func (f fakeSessionReader) Read(*http.Request) (model.Session, error) { return f.sess, nil }

type fakeUserLoader struct{ u model.User }

func (f fakeUserLoader) GetUserByID(context.Context, string) (model.User, error) { return f.u, nil }

type fakeAudit struct{ events []model.AuditEventType }

func (f *fakeAudit) RecordEvent(_ context.Context, _ *string, t model.AuditEventType, _ map[string]any) error {
	f.events = append(f.events, t)
	return nil
}

type memUnits struct{ units []model.Unit }

func (m *memUnits) ListUnitsForCondominium(context.Context, string) ([]model.Unit, error) {
	return m.units, nil
}

func (m *memUnits) CreateUnit(_ context.Context, condo, code string, ppm int) (model.Unit, error) {
	for _, u := range m.units {
		if u.Code == code {
			return model.Unit{}, store.ErrConflict
		}
	}
	u := model.Unit{ID: "n" + code, CondominiumID: condo, Code: code, SharePPM: ppm}
	m.units = append(m.units, u)
	return u, nil
}

func (m *memUnits) UpdateUnit(_ context.Context, _, id, code string, ppm int) error {
	for i := range m.units {
		if m.units[i].ID == id {
			m.units[i].Code, m.units[i].SharePPM = code, ppm
			return nil
		}
	}
	return store.ErrNotFound
}

func unitsRouter(roles []model.Role, mem *memUnits, au *fakeAudit) http.Handler {
	deps := Deps{
		Roles:       &fakeRoles{roles: roles},
		Audit:       au,
		UnitService: &services.UnitService{Units: mem, Audit: au},
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, deps)
	user := model.User{ID: "u1", Email: "syndic@example.com", Status: model.UserStatusActive}
	sess := model.Session{TokenHash: "th", UserID: "u1", CondominiumID: "c1", CSRFToken: "x"}
	var root http.Handler = mux
	root = middleware.WithLocale(root)
	return middleware.WithUser(fakeSessionReader{sess: sess}, fakeUserLoader{u: user})(root)
}

func do(h http.Handler, method, path string, form url.Values) *httptest.ResponseRecorder {
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestUnitsListShowsSumAndDiff(t *testing.T) {
	mem := &memUnits{units: []model.Unit{{ID: "1", Code: "A-1", SharePPM: 400000}, {ID: "2", Code: "B-2", SharePPM: 350500}}}
	rec := do(unitsRouter([]model.Role{model.RoleSyndic}, mem, &fakeAudit{}), http.MethodGet, "/condominium/units", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"A-1", "B-2", "350.5", "750.5", "249.5", "Ownership share"} {
		if !strings.Contains(body, want) {
			t.Errorf("list missing %q", want)
		}
	}
}

func TestUnitsListPaginates(t *testing.T) {
	mem := &memUnits{}
	for i := 1; i <= 25; i++ {
		code := fmt.Sprintf("U%02d", i)
		mem.units = append(mem.units, model.Unit{ID: code, Code: code, SharePPM: 10000})
	}
	r := unitsRouter([]model.Role{model.RoleSyndic}, mem, &fakeAudit{})

	body := do(r, http.MethodGet, "/condominium/units", nil).Body.String()
	if !strings.Contains(body, "U20") || strings.Contains(body, "U21") {
		t.Errorf("page 1 should hold U01..U20 only")
	}
	if !strings.Contains(body, "page=2") {
		t.Errorf("page 1 missing next link")
	}
	// The share sum covers all 25 units (250%), not just the page.
	if !strings.Contains(body, "250") {
		t.Errorf("sum should be computed over all units")
	}

	body = do(r, http.MethodGet, "/condominium/units?page=2&flash=created", nil).Body.String()
	if !strings.Contains(body, "U21") || !strings.Contains(body, "U25") || strings.Contains(body, "U20") {
		t.Errorf("page 2 should hold U21..U25 only")
	}
	if strings.Contains(body, "flash=") {
		t.Errorf("pagination links must drop flash")
	}

	body = do(r, http.MethodGet, "/condominium/units?page=99", nil).Body.String()
	if !strings.Contains(body, "U25") {
		t.Errorf("out of range page should clamp to last page")
	}
}

func TestUnitsCreateAndEdit(t *testing.T) {
	mem := &memUnits{units: []model.Unit{{ID: "1", Code: "A", SharePPM: 100000}}}
	au := &fakeAudit{}
	r := unitsRouter([]model.Role{model.RoleSyndic}, mem, au)

	rec := do(r, http.MethodPost, "/condominium/units", url.Values{"code": {"C"}, "share": {"250"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/condominium/units?flash=created" {
		t.Fatalf("create = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	rec = do(r, http.MethodPost, "/condominium/units/1/edit", url.Values{"code": {"A"}, "share": {"400"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("edit = %d", rec.Code)
	}
	if mem.units[0].SharePPM != 400000 || mem.units[1].SharePPM != 250000 {
		t.Fatalf("units = %+v", mem.units)
	}
	if len(au.events) != 2 || au.events[0] != model.AuditUnitCreated || au.events[1] != model.AuditUnitUpdated {
		t.Fatalf("events = %v", au.events)
	}
	rec = do(r, http.MethodGet, "/condominium/units/1/edit", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `value="400"`) {
		t.Fatalf("edit form = %d", rec.Code)
	}
}

func TestUnitsDuplicateAndInvalidSaveNothing(t *testing.T) {
	mem := &memUnits{units: []model.Unit{{ID: "1", Code: "A", SharePPM: 100000}}}
	au := &fakeAudit{}
	r := unitsRouter([]model.Role{model.RoleSyndic}, mem, au)

	rec := do(r, http.MethodPost, "/condominium/units", url.Values{"code": {"A"}, "share": {"10"}})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Another unit already uses this code.") {
		t.Fatalf("duplicate = %d", rec.Code)
	}
	for _, share := range []string{"-5", "1000.5", "1.2345"} {
		rec = do(r, http.MethodPost, "/condominium/units", url.Values{"code": {"Z"}, "share": {share}})
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "up to three decimals") {
			t.Fatalf("share %q = %d", share, rec.Code)
		}
	}
	if len(mem.units) != 1 || len(au.events) != 0 {
		t.Fatalf("nothing must be saved: units=%+v events=%v", mem.units, au.events)
	}
}

func TestUnitsForbiddenForNonSyndic(t *testing.T) {
	for _, role := range []model.Role{model.RoleOwner, model.RoleTenant} {
		r := unitsRouter([]model.Role{role}, &memUnits{}, &fakeAudit{})
		for _, c := range []struct{ method, path string }{
			{http.MethodGet, "/condominium/units"},
			{http.MethodGet, "/condominium/units/new"},
			{http.MethodPost, "/condominium/units"},
			{http.MethodGet, "/condominium/units/1/edit"},
			{http.MethodPost, "/condominium/units/1/edit"},
		} {
			var form url.Values
			if c.method == http.MethodPost {
				form = url.Values{"code": {"X"}, "share": {"1"}}
			}
			if rec := do(r, c.method, c.path, form); rec.Code != http.StatusForbidden {
				t.Errorf("%s %s as %s = %d, want 403", c.method, c.path, role, rec.Code)
			}
		}
	}
}

// TestRegisterRoutesSmoke fails on ServeMux pattern conflicts, which would
// otherwise panic at startup.
func TestRegisterRoutesSmoke(t *testing.T) {
	RegisterRoutes(http.NewServeMux(), Deps{Roles: &fakeRoles{}, UnitService: &services.UnitService{}})
}
