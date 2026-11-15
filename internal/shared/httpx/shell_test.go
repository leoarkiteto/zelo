package httpx

import (
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

func TestNavForInvoiceEntriesAreSyndicOnly(t *testing.T) {
	want := map[string]string{
		"/invoices":             "nav.invoices",
		"/suppliers":            "nav.suppliers",
		"/condominium/settings": "nav.condominium_settings",
	}
	paths := func(roles []model.Role) map[string]string {
		got := map[string]string{}
		for _, it := range NavFor(roles, "/", i18n.LanguageEN) {
			got[it.Path] = it.Label
		}
		return got
	}

	syndic := paths([]model.Role{model.RoleSyndic})
	for path, key := range want {
		label, ok := syndic[path]
		if !ok {
			t.Fatalf("syndic nav is missing %s", path)
		}
		if label != i18n.T(i18n.LanguageEN, i18n.MessageKey(key)) {
			t.Fatalf("%s label = %q, want the %s message", path, label, key)
		}
	}
	for _, roles := range [][]model.Role{{model.RoleOwner}, {model.RoleTenant}, {model.RoleOwner, model.RoleTenant}} {
		got := paths(roles)
		for path := range want {
			if _, ok := got[path]; ok {
				t.Fatalf("nav for %v must not contain %s", roles, path)
			}
		}
	}
}

func TestNavForHasNoFinanceHealthItem(t *testing.T) {
	for _, roles := range [][]model.Role{{model.RoleOwner}, {model.RoleSyndic}} {
		for _, it := range NavFor(roles, "/", i18n.LanguageEN) {
			if it.Path == "/finance/health" {
				t.Fatalf("nav for %v still contains /finance/health", roles)
			}
		}
	}
}
