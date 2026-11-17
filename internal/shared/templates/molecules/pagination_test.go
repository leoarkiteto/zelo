package molecules

import (
	"net/url"
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/pagination"
	"github.com/leoarkiteto/zelo/internal/shared/templates/testutil"
)

func renderPagination(t *testing.T, l i18n.Language, page, total int, q url.Values) string {
	t.Helper()
	p := pagination.New(page, total)
	return testutil.RenderToString(t, Pagination(PaginationFrom(l, p, "/finance", q, "page")))
}

func TestPaginationHiddenForSinglePage(t *testing.T) {
	for _, total := range []int{0, 1, 20} {
		if html := renderPagination(t, i18n.LanguagePTBR, 1, total, nil); strings.Contains(html, "<nav") {
			t.Errorf("total %d: expected no nav, got %q", total, html)
		}
	}
}

func TestPaginationFirstMiddleLast(t *testing.T) {
	q := url.Values{"status": {"open"}, "flash": {"x"}}
	first := renderPagination(t, i18n.LanguagePTBR, 1, 50, q)
	if strings.Contains(first, "Anterior") || !strings.Contains(first, "Próxima") || !strings.Contains(first, "Página 1 de 3") {
		t.Errorf("first page: %q", first)
	}
	if !strings.Contains(first, `href="/finance?page=2&amp;status=open"`) || strings.Contains(first, "flash") {
		t.Errorf("first page href: %q", first)
	}
	mid := renderPagination(t, i18n.LanguagePTBR, 2, 50, q)
	if !strings.Contains(mid, "Anterior") || !strings.Contains(mid, "Próxima") {
		t.Errorf("middle page: %q", mid)
	}
	last := renderPagination(t, i18n.LanguagePTBR, 3, 50, q)
	if !strings.Contains(last, "Anterior") || strings.Contains(last, "Próxima") {
		t.Errorf("last page: %q", last)
	}
}

func TestPaginationLocales(t *testing.T) {
	cases := map[i18n.Language][]string{
		i18n.LanguageEN:   {"Page 2 of 3", "Previous", "Next", `aria-label="Pagination"`},
		i18n.LanguagePTBR: {"Página 2 de 3", "Anterior", "Próxima", `aria-label="Paginação"`},
		i18n.LanguagePTPT: {"Página 2 de 3", "Anterior", "Seguinte", `aria-label="Paginação"`},
	}
	for l, wants := range cases {
		html := renderPagination(t, l, 2, 50, nil)
		for _, w := range wants {
			if !strings.Contains(html, w) {
				t.Errorf("%s missing %q in %q", l, w, html)
			}
		}
	}
}
