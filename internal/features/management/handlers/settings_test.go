package handlers

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/features/management/services"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/money"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

type memCondo struct {
	currency money.Currency
	lateFee  model.LateFeeSettings
}

func (m *memCondo) GetCurrency(context.Context, string) (money.Currency, error) {
	return m.currency, nil
}

func (m *memCondo) GetLateFeeSettings(context.Context, string) (model.LateFeeSettings, error) {
	return m.lateFee, nil
}

func (m *memCondo) SetLateFeeSettings(_ context.Context, _ string, v model.LateFeeSettings, maxBRLFeeBP int) error {
	if m.currency == money.BRL && v.FeeBP > maxBRLFeeBP {
		return store.ErrLateFeeAboveBRLCap
	}
	m.lateFee = v
	return nil
}

func (m *memCondo) SetCurrencyClampingLateFee(_ context.Context, _ string, c money.Currency, maxBP int) error {
	m.currency = c
	if c == money.BRL && m.lateFee.FeeBP > maxBP {
		m.lateFee.FeeBP = maxBP
	}
	return nil
}

func settingsRouter(roles []model.Role, mem *memCondo, au *fakeAudit) http.Handler {
	deps := Deps{
		Roles:           &fakeRoles{roles: roles},
		Audit:           au,
		Condominiums:    mem,
		SettingsService: &services.SettingsService{Condominiums: mem, Audit: au},
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, deps)
	user := model.User{ID: "u1", Email: "syndic@example.com", Status: model.UserStatusActive}
	sess := model.Session{TokenHash: "th", UserID: "u1", CondominiumID: "c1", CSRFToken: "x"}
	var root http.Handler = mux
	root = middleware.WithLocale(root)
	return middleware.WithUser(fakeSessionReader{sess: sess}, fakeUserLoader{u: user})(root)
}

func TestSettingsShowsLateFeeForm(t *testing.T) {
	mem := &memCondo{currency: money.BRL, lateFee: model.LateFeeSettings{FeeBP: 200, InterestBP: 150, GraceDays: 3}}
	rec := do(settingsRouter([]model.Role{model.RoleSyndic}, mem, &fakeAudit{}), http.MethodGet, settingsPath, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`name="late_fee"`, `value="2"`, `value="1.5"`, `value="3"`, `name="csrf_token"`} {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %q", want)
		}
	}
}

func TestSettingsLateFeeBRLCapRefused(t *testing.T) {
	mem := &memCondo{currency: money.BRL, lateFee: model.LateFeeSettings{FeeBP: 200, InterestBP: 100}}
	au := &fakeAudit{}
	r := settingsRouter([]model.Role{model.RoleSyndic}, mem, au)

	rec := do(r, http.MethodPost, settingsPath, url.Values{"form": {"late_fee"}, "late_fee": {"2,5"}, "late_interest": {"1"}, "late_grace_days": {"0"}})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "cannot exceed 2%") {
		t.Fatalf("status = %d", rec.Code)
	}
	if mem.lateFee.FeeBP != 200 || len(au.events) != 0 {
		t.Fatalf("refused save must not write: %+v", mem.lateFee)
	}

	rec = do(r, http.MethodPost, settingsPath, url.Values{"form": {"late_fee"}, "late_fee": {"2"}, "late_interest": {"1"}, "late_grace_days": {"7"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != settingsPath+"?flash=late_fee_saved" {
		t.Fatalf("save = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if mem.lateFee.GraceDays != 7 || len(au.events) != 1 || au.events[0] != model.AuditLateFeeSettingsChanged {
		t.Fatalf("stored = %+v, events = %v", mem.lateFee, au.events)
	}
}

func TestSettingsLateFeeEURAcceptsFive(t *testing.T) {
	mem := &memCondo{currency: money.EUR}
	rec := do(settingsRouter([]model.Role{model.RoleSyndic}, mem, &fakeAudit{}), http.MethodPost, settingsPath,
		url.Values{"form": {"late_fee"}, "late_fee": {"5"}, "late_interest": {"0"}, "late_grace_days": {"0"}})
	if rec.Code != http.StatusSeeOther || mem.lateFee.FeeBP != 500 {
		t.Fatalf("status = %d, stored = %+v", rec.Code, mem.lateFee)
	}
}

func TestSettingsLateFeeForbiddenForNonSyndic(t *testing.T) {
	mem := &memCondo{currency: money.EUR}
	r := settingsRouter([]model.Role{model.RoleOwner}, mem, &fakeAudit{})
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec := do(r, method, settingsPath, url.Values{"form": {"late_fee"}, "late_fee": {"5"}, "late_interest": {"0"}, "late_grace_days": {"0"}})
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s status = %d, want 403", method, rec.Code)
		}
	}
	if mem.lateFee.FeeBP != 0 {
		t.Fatal("forbidden request must not write")
	}
}
