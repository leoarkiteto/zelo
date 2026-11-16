package httpx

import (
	"reflect"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/templates/organisms"
)

// flatNav walks the two-level nav tree and returns every link (groups
// themselves carry no path and are skipped).
func flatNav(items []organisms.NavItem) []organisms.NavItem {
	var out []organisms.NavItem
	for _, it := range items {
		if len(it.Children) > 0 {
			out = append(out, flatNav(it.Children)...)
			continue
		}
		out = append(out, it)
	}
	return out
}

func navPaths(items []organisms.NavItem) []string {
	var out []string
	for _, it := range flatNav(items) {
		out = append(out, it.Path)
	}
	return out
}

func TestNavForInvoiceEntriesAreSyndicOnly(t *testing.T) {
	want := map[string]string{
		"/invoices":             "nav.invoices",
		"/suppliers":            "nav.suppliers",
		"/condominium/settings": "nav.condominium_settings",
		"/condominium/units":    "nav.units",
	}
	paths := func(roles []model.Role) map[string]string {
		got := map[string]string{}
		for _, it := range flatNav(NavFor(roles, "/", i18n.LanguageEN)) {
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

func TestNavForAccountStatementLabel(t *testing.T) {
	for lang, want := range map[i18n.Language]string{
		i18n.LanguageEN:   "Account statement",
		i18n.LanguagePTBR: "Conta corrente",
		i18n.LanguagePTPT: "Conta corrente",
	} {
		found := false
		for _, it := range flatNav(NavFor([]model.Role{model.RoleOwner}, "/", lang)) {
			if it.Path == "/finance/my-charges" {
				found = true
				if it.Label != want {
					t.Errorf("%s label = %q, want %q", lang, it.Label, want)
				}
			}
		}
		if !found {
			t.Errorf("%s nav is missing /finance/my-charges", lang)
		}
	}
}

func TestNavForHasNoFinanceHealthItem(t *testing.T) {
	for _, roles := range [][]model.Role{{model.RoleOwner}, {model.RoleSyndic}} {
		for _, it := range flatNav(NavFor(roles, "/", i18n.LanguageEN)) {
			if it.Path == "/finance/health" {
				t.Fatalf("nav for %v still contains /finance/health", roles)
			}
		}
	}
}

// AC-1: the syndic menu is grouped by relation, and every page is reachable
// exactly once.
func TestNavForGroupsSyndicEntries(t *testing.T) {
	nav := NavFor([]model.Role{model.RoleSyndic}, "/", i18n.LanguageEN)

	type entry struct {
		label    string
		path     string
		children []string
	}
	want := []entry{
		{label: "Dashboard", path: "/"},
		{label: "Service directory", path: "/directory"},
		{label: "Finance", children: []string{"/finance/my-charges", "/finance", "/invoices", "/suppliers"}},
		{label: "Tickets", children: []string{"/tickets", "/tickets/inbox"}},
		{label: "Condominium", children: []string{"/condominium", "/condominium/settings", "/condominium/units", "/invitations", "/roles"}},
		{label: "My unit", path: "/unit"},
	}
	if len(nav) != len(want) {
		t.Fatalf("syndic top level has %d entries, want %d: %+v", len(nav), len(want), nav)
	}
	for i, w := range want {
		got := nav[i]
		if got.Label != w.label || got.Path != w.path {
			t.Errorf("entry %d = %q (%q), want %q (%q)", i, got.Label, got.Path, w.label, w.path)
		}
		var childPaths []string
		for _, c := range got.Children {
			childPaths = append(childPaths, c.Path)
		}
		if !reflect.DeepEqual(childPaths, w.children) {
			t.Errorf("%s children = %v, want %v", w.label, childPaths, w.children)
		}
	}

	seen := map[string]int{}
	for _, p := range navPaths(nav) {
		seen[p]++
	}
	for p, n := range seen {
		if n != 1 {
			t.Errorf("path %s appears %d times", p, n)
		}
	}
}

// AC-2: owners and tenants keep a flat menu with the same pages as before.
func TestNavForOwnerAndTenantStayFlat(t *testing.T) {
	cases := map[string]struct {
		roles []model.Role
		want  []string
	}{
		"owner":        {[]model.Role{model.RoleOwner}, []string{"/", "/directory", "/finance/my-charges", "/tickets", "/unit"}},
		"tenant":       {[]model.Role{model.RoleTenant}, []string{"/", "/directory", "/finance/my-charges", "/tickets", "/tenancy"}},
		"owner+tenant": {[]model.Role{model.RoleOwner, model.RoleTenant}, []string{"/", "/directory", "/finance/my-charges", "/tickets", "/unit", "/tenancy"}},
	}
	for name, tc := range cases {
		nav := NavFor(tc.roles, "/", i18n.LanguageEN)
		var got []string
		for _, it := range nav {
			if len(it.Children) > 0 {
				t.Errorf("%s nav must have no groups, got %q", name, it.Label)
			}
			got = append(got, it.Path)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s nav = %v, want %v", name, got, tc.want)
		}
	}
}

// FR-3: the group holding the current page is active (renders open) and only
// that child is highlighted.
func TestNavForMarksActiveGroup(t *testing.T) {
	nav := NavFor([]model.Role{model.RoleSyndic}, "/invoices", i18n.LanguageEN)
	for _, it := range nav {
		wantOpen := it.Label == "Finance"
		if it.Active != wantOpen {
			t.Errorf("%s Active = %v, want %v", it.Label, it.Active, wantOpen)
		}
	}
	for _, it := range flatNav(nav) {
		if it.Active != (it.Path == "/invoices") {
			t.Errorf("%s Active = %v", it.Path, it.Active)
		}
	}

	// A top-level page opens no group.
	for _, it := range NavFor([]model.Role{model.RoleSyndic}, "/unit", i18n.LanguageEN) {
		if len(it.Children) > 0 && it.Active {
			t.Errorf("group %s must be closed on /unit", it.Label)
		}
	}
}

// AC-4: group labels are translated in every supported language.
func TestNavForGroupLabelsAreTranslated(t *testing.T) {
	for lang, want := range map[i18n.Language][]string{
		i18n.LanguageEN:   {"Finance", "Tickets", "Condominium"},
		i18n.LanguagePTBR: {"Financeiro", "Chamados", "Condomínio"},
		i18n.LanguagePTPT: {"Financeiro", "Pedidos", "Condomínio"},
	} {
		var got []string
		for _, it := range NavFor([]model.Role{model.RoleSyndic}, "/", lang) {
			if len(it.Children) > 0 {
				got = append(got, it.Label)
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s group labels = %v, want %v", lang, got, want)
		}
	}
	for lang, want := range map[i18n.Language]string{
		i18n.LanguageEN:   "Overview",
		i18n.LanguagePTBR: "Visão geral",
		i18n.LanguagePTPT: "Visão geral",
	} {
		if got := i18n.T(lang, "nav.finance.overview"); got != want {
			t.Errorf("%s nav.finance.overview = %q, want %q", lang, got, want)
		}
	}
}
