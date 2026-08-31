package atoms

import (
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/templates/testutil"
)

func TestButtonRendersAnchor(t *testing.T) {
	html := testutil.RenderToString(t, Button(ButtonProps{Label: "Save", Variant: ButtonVariantPrimary, Href: "/save"}))
	for _, want := range []string{`class="btn btn-primary"`, `href="/save"`, "Save"} {
		if !strings.Contains(html, want) {
			t.Errorf("Button anchor is missing %q in %q", want, html)
		}
	}
}

func TestButtonRendersSubmit(t *testing.T) {
	html := testutil.RenderToString(t, Button(ButtonProps{Label: "Go", Variant: ButtonVariantSecondary, Type: "submit", Class: "btn-sm"}))
	for _, want := range []string{`class="btn btn-secondary btn-sm"`, `type="submit"`, "Go"} {
		if !strings.Contains(html, want) {
			t.Errorf("Button submit is missing %q in %q", want, html)
		}
	}
}

func TestBadgeRendersVariant(t *testing.T) {
	html := testutil.RenderToString(t, Badge(BadgeProps{Label: "syndic", Variant: BadgeVariantWarning}))
	for _, want := range []string{`class="badge badge-warning"`, "syndic"} {
		if !strings.Contains(html, want) {
			t.Errorf("Badge is missing %q in %q", want, html)
		}
	}
}

func TestIconRendersKnownKind(t *testing.T) {
	html := testutil.RenderToString(t, Icon(IconProps{Kind: IconKindInfo}))
	for _, want := range []string{"<svg", `class="h-5 w-5"`} {
		if !strings.Contains(html, want) {
			t.Errorf("Icon is missing %q in %q", want, html)
		}
	}
}

func TestTextInputRendersRequiredAttribute(t *testing.T) {
	html := testutil.RenderToString(t, TextInput(InputProps{Type: "email", ID: "email", Name: "email", Placeholder: "you@example.com", Required: true}))
	for _, want := range []string{`class="input"`, `type="email"`, `id="email"`, `name="email"`, `placeholder="you@example.com"`, "required"} {
		if !strings.Contains(html, want) {
			t.Errorf("TextInput is missing %q in %q", want, html)
		}
	}
}

func TestTextAreaRendersValue(t *testing.T) {
	html := testutil.RenderToString(t, TextArea(InputProps{ID: "notes", Name: "notes", Value: "hello"}))
	for _, want := range []string{`class="input"`, `id="notes"`, `name="notes"`, "hello"} {
		if !strings.Contains(html, want) {
			t.Errorf("TextArea is missing %q in %q", want, html)
		}
	}
}

func TestSelectRendersOptions(t *testing.T) {
	html := testutil.RenderToString(t, Select(SelectProps{
		ID:       "role",
		Name:     "role",
		Options:  []SelectOption{{Value: "tenant", Label: "Tenant"}, {Value: "syndic", Label: "Syndic"}},
		Selected: "syndic",
	}))
	for _, want := range []string{`class="select"`, `id="role"`, `name="role"`, "Tenant", "Syndic", "selected"} {
		if !strings.Contains(html, want) {
			t.Errorf("Select is missing %q in %q", want, html)
		}
	}
}
