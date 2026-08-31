package molecules

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/a-h/templ"
	atoms "github.com/leoarkiteto/zelo/internal/shared/templates/atoms"
	"github.com/leoarkiteto/zelo/internal/shared/templates/testutil"
)

func componentFunc(body string) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, body)
		return err
	})
}

func TestCardRendersTitleAndBody(t *testing.T) {
	html := testutil.RenderToString(t, Card(CardProps{Title: "Summary", Body: componentFunc("<p>body</p>")}))
	for _, want := range []string{`class="card card-pad"`, "Summary", "<p>body</p>"} {
		if !strings.Contains(html, want) {
			t.Errorf("Card is missing %q in %q", want, html)
		}
	}
}

func TestFormFieldRendersLabelAndError(t *testing.T) {
	html := testutil.RenderToString(t, FormField(FormFieldProps{
		Label:   "Email",
		For:     "email",
		Control: atoms.TextInput(atoms.InputProps{Type: "email", ID: "email", Name: "email"}),
		Error:   "Required",
	}))
	for _, want := range []string{`class="label"`, `for="email"`, "Email", `type="email"`, "Required"} {
		if !strings.Contains(html, want) {
			t.Errorf("FormField is missing %q in %q", want, html)
		}
	}
}

func TestAlertRendersVariant(t *testing.T) {
	html := testutil.RenderToString(t, Alert(AlertProps{Variant: AlertVariantError, Message: "Denied"}))
	for _, want := range []string{`class="alert alert-error"`, "Denied"} {
		if !strings.Contains(html, want) {
			t.Errorf("Alert is missing %q in %q", want, html)
		}
	}
}

func TestEmptyStateRendersParts(t *testing.T) {
	html := testutil.RenderToString(t, EmptyState(EmptyStateProps{
		Icon:   atoms.IconKindInfo,
		Title:  "Nothing here",
		Copy:   "Try again later",
		Action: atoms.Button(atoms.ButtonProps{Label: "Add", Variant: atoms.ButtonVariantPrimary, Href: "/new"}),
	}))
	for _, want := range []string{`class="empty-state"`, "Nothing here", "Try again later", `class="btn btn-primary"`, "Add"} {
		if !strings.Contains(html, want) {
			t.Errorf("EmptyState is missing %q in %q", want, html)
		}
	}
}
