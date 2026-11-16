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

	"settings.late_fee.title":         "Late payment",
	"settings.late_fee.hint":          "Penalty and interest charged on overdue quotas.",
	"settings.late_fee.fee":           "Late fee (%)",
	"settings.late_fee.fee_hint":      "One-time penalty on the overdue amount, from 0 to 100 with up to two decimals.",
	"settings.late_fee.cap_hint":      "In BRL the late fee cannot exceed 2%.",
	"settings.late_fee.interest":      "Interest (% per month)",
	"settings.late_fee.interest_hint": "Monthly interest on the overdue amount, from 0 to 100 with up to two decimals.",
	"settings.late_fee.grace":         "Grace days",
	"settings.late_fee.grace_hint":    "Days after the due date before late charges apply (0 to 60).",
	"settings.flash.late_fee_saved":   "Late payment settings saved.",
	"settings.error.invalid_fee":      "Enter a late fee from 0 to 100 with up to two decimals.",
	"settings.error.invalid_interest": "Enter an interest rate from 0 to 100 with up to two decimals.",
	"settings.error.invalid_grace":    "Enter grace days from 0 to 60.",
	"settings.error.fee_above_cap":    "In BRL the late fee cannot exceed 2%.",
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

	"settings.late_fee.title":         "Atraso no pagamento",
	"settings.late_fee.hint":          "Multa e juros cobrados sobre quotas em atraso.",
	"settings.late_fee.fee":           "Multa (%)",
	"settings.late_fee.fee_hint":      "Multa única sobre o valor em atraso, de 0 a 100 com até duas casas decimais.",
	"settings.late_fee.cap_hint":      "Em BRL a multa não pode exceder 2%.",
	"settings.late_fee.interest":      "Juros (% ao mês)",
	"settings.late_fee.interest_hint": "Juros mensais sobre o valor em atraso, de 0 a 100 com até duas casas decimais.",
	"settings.late_fee.grace":         "Dias de carência",
	"settings.late_fee.grace_hint":    "Dias após o vencimento antes de cobrar encargos (0 a 60).",
	"settings.flash.late_fee_saved":   "Configurações de atraso salvas.",
	"settings.error.invalid_fee":      "Informe uma multa de 0 a 100 com até duas casas decimais.",
	"settings.error.invalid_interest": "Informe juros de 0 a 100 com até duas casas decimais.",
	"settings.error.invalid_grace":    "Informe dias de carência de 0 a 60.",
	"settings.error.fee_above_cap":    "Em BRL a multa não pode exceder 2%.",
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

	"settings.late_fee.title":         "Atraso no pagamento",
	"settings.late_fee.hint":          "Multa e juros cobrados sobre quotas em atraso.",
	"settings.late_fee.fee":           "Multa (%)",
	"settings.late_fee.fee_hint":      "Multa única sobre o valor em atraso, de 0 a 100 com até duas casas decimais.",
	"settings.late_fee.cap_hint":      "Em BRL a multa não pode exceder 2%.",
	"settings.late_fee.interest":      "Juros (% ao mês)",
	"settings.late_fee.interest_hint": "Juros mensais sobre o valor em atraso, de 0 a 100 com até duas casas decimais.",
	"settings.late_fee.grace":         "Dias de carência",
	"settings.late_fee.grace_hint":    "Dias após o vencimento antes de aplicar encargos (0 a 60).",
	"settings.flash.late_fee_saved":   "Definições de atraso guardadas.",
	"settings.error.invalid_fee":      "Introduza uma multa de 0 a 100 com até duas casas decimais.",
	"settings.error.invalid_interest": "Introduza juros de 0 a 100 com até duas casas decimais.",
	"settings.error.invalid_grace":    "Introduza dias de carência de 0 a 60.",
	"settings.error.fee_above_cap":    "Em BRL a multa não pode exceder 2%.",
}
