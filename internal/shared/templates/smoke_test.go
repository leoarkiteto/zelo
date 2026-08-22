package templates

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// renderToString renders a component and fails the test on any error.
func renderToString(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

// TestPagesRender guards every shared page against render panics and checks
// that the core structure of each page is present in the output.
func TestPagesRender(t *testing.T) {
	cases := []struct {
		name string
		page templ.Component
		want []string
	}{
		{"error",
			ErrorPage(ErrorPageData{Status: 403, Message: "Access denied."}),
			[]string{"403", "Access denied.", "Go to dashboard"}},
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
