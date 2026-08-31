package molecules

import (
	"github.com/a-h/templ"
	atoms "github.com/leoarkiteto/zelo/internal/shared/templates/atoms"
)

// AlertVariant maps to the existing Tailwind `alert alert-<variant>` classes.
type AlertVariant string

const (
	AlertVariantInfo    AlertVariant = "info"
	AlertVariantSuccess AlertVariant = "success"
	AlertVariantError   AlertVariant = "error"
)

// AlertProps carries everything Alert needs to render.
type AlertProps struct {
	Variant AlertVariant
	Message string
	Class   string
}

func alertClass(p AlertProps) string {
	c := "alert"
	if p.Variant != "" {
		c += " alert-" + string(p.Variant)
	}
	if p.Class != "" {
		c += " " + p.Class
	}
	return c
}

// CardProps carries everything Card needs to render.
type CardProps struct {
	Title  string
	Body   templ.Component
	Footer templ.Component
	Class  string
}

func cardClass(p CardProps) string {
	c := "card card-pad"
	if p.Class != "" {
		c += " " + p.Class
	}
	return c
}

// FormFieldProps carries everything FormField needs to render.
type FormFieldProps struct {
	Label   string
	For     string
	Control templ.Component
	Error   string
	Hint    string
}

// EmptyStateProps carries everything EmptyState needs to render.
type EmptyStateProps struct {
	Icon   atoms.IconKind
	Title  string
	Copy   string
	Action templ.Component
}
