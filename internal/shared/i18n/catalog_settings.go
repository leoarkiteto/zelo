package i18n

// Settings area: the condominium currency settings page (settings.*) and its
// nav label (nav.condominium_settings).
func init() {
	registerArea("settings", enSettings, ptbrSettings, ptptSettings)
}

var enSettings = map[MessageKey]string{
	"nav.condominium_settings":        "Condominium settings",
	"settings.title":                  "Condominium settings",
	"settings.subtitle":               "Choose the currency used for every amount of the condominium.",
	"settings.currency":               "Currency",
	"settings.currency.eur":           "Euro (EUR)",
	"settings.currency.brl":           "Brazilian real (BRL)",
	"settings.currency_hint":          "Applies to invoices, finance and every other amount shown for this condominium.",
	"settings.currency_warning":       "Changing the currency only changes how amounts are displayed. Existing values are not converted.",
	"settings.flash.currency_changed": "Currency changed.",
	"settings.error.invalid_currency": "Choose a valid currency.",
	"settings.error.save_failed":      "The settings could not be saved.",
}

var ptbrSettings = map[MessageKey]string{
	"nav.condominium_settings":        "Configurações do condomínio",
	"settings.title":                  "Configurações do condomínio",
	"settings.subtitle":               "Escolha a moeda usada em todos os valores do condomínio.",
	"settings.currency":               "Moeda",
	"settings.currency.eur":           "Euro (EUR)",
	"settings.currency.brl":           "Real brasileiro (BRL)",
	"settings.currency_hint":          "Vale para faturas, finanças e todos os outros valores exibidos deste condomínio.",
	"settings.currency_warning":       "Mudar a moeda só altera a forma como os valores são exibidos. Os valores existentes não são convertidos.",
	"settings.flash.currency_changed": "Moeda alterada.",
	"settings.error.invalid_currency": "Escolha uma moeda válida.",
	"settings.error.save_failed":      "Não foi possível salvar as configurações.",
}

var ptptSettings = map[MessageKey]string{
	"nav.condominium_settings":        "Definições do condomínio",
	"settings.title":                  "Definições do condomínio",
	"settings.subtitle":               "Escolha a moeda usada em todos os valores do condomínio.",
	"settings.currency":               "Moeda",
	"settings.currency.eur":           "Euro (EUR)",
	"settings.currency.brl":           "Real brasileiro (BRL)",
	"settings.currency_hint":          "Aplica-se às faturas, às finanças e a todos os outros valores apresentados deste condomínio.",
	"settings.currency_warning":       "Alterar a moeda só muda a forma como os valores são apresentados. Os valores existentes não são convertidos.",
	"settings.flash.currency_changed": "Moeda alterada.",
	"settings.error.invalid_currency": "Escolha uma moeda válida.",
	"settings.error.save_failed":      "Não foi possível guardar as definições.",
}
