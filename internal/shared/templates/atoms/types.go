package atoms

import "github.com/a-h/templ"

// ButtonVariant maps to the existing Tailwind `btn btn-<variant>` classes.
type ButtonVariant string

const (
	ButtonVariantPrimary   ButtonVariant = "primary"
	ButtonVariantSecondary ButtonVariant = "secondary"
	ButtonVariantDanger    ButtonVariant = "danger"
	ButtonVariantGhost     ButtonVariant = "ghost"
	ButtonVariantOutline   ButtonVariant = "outline"
)

// ButtonProps carries everything Button needs to render a link or button.
type ButtonProps struct {
	Label   string
	Variant ButtonVariant
	Type    string // submit, button; ignored when Href is set
	Href    string // renders an anchor when non-empty
	Class   string // extra classes, e.g. btn-sm, btn-block
}

func buttonClass(p ButtonProps) string {
	c := "btn"
	if p.Variant != "" {
		c += " btn-" + string(p.Variant)
	}
	if p.Class != "" {
		c += " " + p.Class
	}
	return c
}

func buttonType(p ButtonProps) string {
	if p.Type == "" {
		return "button"
	}
	return p.Type
}

// BadgeVariant maps to the existing Tailwind `badge badge-<variant>` classes.
type BadgeVariant string

const (
	BadgeVariantInfo    BadgeVariant = "info"
	BadgeVariantSuccess BadgeVariant = "success"
	BadgeVariantWarning BadgeVariant = "warning"
	BadgeVariantDanger  BadgeVariant = "danger"
	BadgeVariantNeutral BadgeVariant = "neutral"
)

// BadgeProps carries everything Badge needs to render.
type BadgeProps struct {
	Label   string
	Variant BadgeVariant
	Class   string
}

func badgeClass(p BadgeProps) string {
	c := "badge"
	if p.Variant != "" {
		c += " badge-" + string(p.Variant)
	}
	if p.Class != "" {
		c += " " + p.Class
	}
	return c
}

// IconKind identifies one shared SVG icon.
type IconKind string

const (
	IconKindBuilding     IconKind = "building"
	IconKindMail         IconKind = "mail"
	IconKindUsers        IconKind = "users"
	IconKindHome         IconKind = "home"
	IconKindKey          IconKind = "key"
	IconKindInfo         IconKind = "info"
	IconKindChevronLeft  IconKind = "chevron-left"
	IconKindArrowRight   IconKind = "arrow-right"
	IconKindClipboard    IconKind = "clipboard"
	IconKindTag          IconKind = "tag"
)

// IconProps carries everything Icon needs to render.
type IconProps struct {
	Kind  IconKind
	Class string // size/color overrides; defaults to "h-5 w-5"
}

func iconClass(p IconProps) string {
	if p.Class != "" {
		return p.Class
	}
	return "h-5 w-5"
}

// InputProps carries the common attributes for form controls.
type InputProps struct {
	Type         string // text, email, password, tel, date, number...
	ID           string
	Name         string
	Value        string
	Placeholder  string
	Autocomplete string
	MinLength    string
	MaxLength    string
	InputMode    string
	Rows         string // textarea only
	Required     bool
	Class        string
}

func inputClass(p InputProps) string {
	c := "input"
	if p.Class != "" {
		c += " " + p.Class
	}
	return c
}

func selectClass(p SelectProps) string {
	c := "select"
	if p.Class != "" {
		c += " " + p.Class
	}
	return c
}

func inputType(p InputProps) string {
	if p.Type == "" {
		return "text"
	}
	return p.Type
}

func inputAttrs(p InputProps) templ.Attributes {
	a := templ.Attributes{}
	if p.Placeholder != "" {
		a["placeholder"] = p.Placeholder
	}
	if p.Autocomplete != "" {
		a["autocomplete"] = p.Autocomplete
	}
	if p.MinLength != "" {
		a["minlength"] = p.MinLength
	}
	if p.MaxLength != "" {
		a["maxlength"] = p.MaxLength
	}
	if p.InputMode != "" {
		a["inputmode"] = p.InputMode
	}
	if p.Rows != "" {
		a["rows"] = p.Rows
	}
	if p.Required {
		a["required"] = true
	}
	return a
}

// SelectOption is one option in a Select.
type SelectOption struct {
	Value string
	Label string
}

// SelectProps carries everything Select needs to render.
type SelectProps struct {
	ID       string
	Name     string
	Options  []SelectOption
	Selected string
	Required bool
	Class    string
}

func selectAttrs(p SelectProps) templ.Attributes {
	a := templ.Attributes{}
	if p.Required {
		a["required"] = true
	}
	return a
}
