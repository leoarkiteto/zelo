package templates

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	organisms "github.com/leoarkiteto/zelo/internal/shared/templates/organisms"
)

func renderToString(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func shellFixture() *organisms.ShellData {
	return &organisms.ShellData{
		User: &organisms.UserView{Email: "anna@example.com", Roles: []string{"syndic"}, Initial: "A"},
		Nav:  []organisms.NavItem{{Label: "Dashboard", Path: "/", Active: true}},
		CSRF: "tok",
	}
}

// TestPagesRender guards every directory page against render panics and
// checks that the core structure of each page is present in the output.
func TestPagesRender(t *testing.T) {
	cases := []struct {
		name string
		page templ.Component
		want []string
	}{
		{"directory",
			DirectoryPage(DirectoryPageData{
				Shell: shellFixture(), CSRF: "tok",
				Listings:   []ListingView{{ID: "l1", Name: "Ana Plumbing", CategoryName: "Plumber", Phone: "+55 11 91234-5678", Notes: "Fast", UnitCode: "A-101"}},
				Categories: []CategoryOption{{ID: "c1", Name: "Plumber"}},
				IsSyndic:   true,
				Flash:      "Listing added to the directory."}),
			[]string{"Service directory", "Ana Plumbing", "Recommend a professional", "Manage categories", "Unit A-101", "Listing added to the directory."}},
		{"directory-empty",
			DirectoryPage(DirectoryPageData{Shell: shellFixture(), CSRF: "tok"}),
			[]string{"No listings found"}},
		{"listing-form",
			ListingFormPage(ListingFormData{Shell: shellFixture(), CSRF: "tok",
				Categories: []CategoryOption{{ID: "c1", Name: "Plumber"}}}),
			[]string{"Recommend a professional", "Add to directory", "Phone number"}},
		{"listing-form-duplicate",
			ListingFormPage(ListingFormData{Shell: shellFixture(), CSRF: "tok",
				Categories:       []CategoryOption{{ID: "c1", Name: "Plumber"}},
				DuplicateWarning: "A listing with this phone number already exists."}),
			[]string{"already exists", "confirm_duplicate"}},
		{"categories",
			CategoriesPage(CategoriesPageData{Shell: shellFixture(), CSRF: "tok",
				Categories: []CategoryView{{ID: "c1", Name: "Plumber", Active: true}, {ID: "c2", Name: "Old", Active: false}}}),
			[]string{"Directory categories", "Rename", "Deactivate", "deactivated"}},
		{"delete-confirm",
			DeleteConfirmPage(DeleteConfirmData{Shell: shellFixture(), CSRF: "tok",
				Listing: ListingView{ID: "l1", Name: "Ana Plumbing", CategoryName: "Plumber", Phone: "+55", UnitCode: "A-101"}}),
			[]string{"Delete listing", "Cancel"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			html := renderToString(t, c.page)
			for _, want := range c.want {
				if !strings.Contains(html, want) {
					t.Errorf("rendered %s is missing %q", c.name, want)
				}
			}
		})
	}
}
