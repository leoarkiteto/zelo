// Package domain holds the pure-Go rules of the invoices feature: enums, the
// VAT table by fiscal region, NIF validation, line and total computation in
// integer cents and basis points, payment status derivation and the payment,
// edit and annulment rules.
//
// Vocabulary follows GLOSSARY.md: Fatura, Fornecedor, Linha da fatura,
// Retenção na fonte, Valor a pagar, Valor em dívida, Pagamento da fatura,
// Rascunho and Anulação.
//
// Money is int64 cents, VAT rates are integer basis points (2300 = 23 %) and
// quantities are integer milli-units (2.5 -> 2500). No floating point is used
// anywhere in this package (NFR-1).
package domain

// DocumentType is the kind of fiscal document received from a fornecedor.
type DocumentType string

const (
	DocumentFatura             DocumentType = "fatura"
	DocumentFaturaRecibo       DocumentType = "fatura_recibo"
	DocumentFaturaSimplificada DocumentType = "fatura_simplificada"
	// DocumentNotaCredito is reserved for phase 2 (FR-34): it is a valid
	// stored value but cannot be registered yet, see Registrable.
	DocumentNotaCredito DocumentType = "nota_credito"
	DocumentNotaDebito  DocumentType = "nota_debito"
	DocumentOutro       DocumentType = "outro"
)

// AllDocumentTypes lists every stored document type, nota_credito included.
func AllDocumentTypes() []DocumentType {
	return []DocumentType{
		DocumentFatura, DocumentFaturaRecibo, DocumentFaturaSimplificada,
		DocumentNotaCredito, DocumentNotaDebito, DocumentOutro,
	}
}

// RegistrableDocumentTypes lists the types the UI offers in phase 1.
func RegistrableDocumentTypes() []DocumentType {
	var out []DocumentType
	for _, d := range AllDocumentTypes() {
		if d.Registrable() {
			out = append(out, d)
		}
	}
	return out
}

// Valid reports whether d is a known stored value.
func (d DocumentType) Valid() bool {
	for _, v := range AllDocumentTypes() {
		if d == v {
			return true
		}
	}
	return false
}

// Registrable reports whether d may be used to register a fatura in phase 1.
// It excludes nota_credito (FR-34).
func (d DocumentType) Registrable() bool { return d.Valid() && d != DocumentNotaCredito }

// LabelKey is the i18n key of the display name.
func (d DocumentType) LabelKey() string { return "invoices.document_type." + string(d) }

// ExpenseType is the nature of the expense on the fatura.
type ExpenseType string

const (
	ExpenseCompraProdutos        ExpenseType = "compra_produtos"
	ExpensePrestacaoServicos     ExpenseType = "prestacao_servicos"
	ExpenseDespesaMensal         ExpenseType = "despesa_mensal"
	ExpenseDespesaAdministrativa ExpenseType = "despesa_administrativa"
	ExpenseManutencao            ExpenseType = "manutencao"
	ExpenseOutraDespesa          ExpenseType = "outra_despesa"
)

// AllExpenseTypes lists the six expense types.
func AllExpenseTypes() []ExpenseType {
	return []ExpenseType{
		ExpenseCompraProdutos, ExpensePrestacaoServicos, ExpenseDespesaMensal,
		ExpenseDespesaAdministrativa, ExpenseManutencao, ExpenseOutraDespesa,
	}
}

// Valid reports whether e is a known value.
func (e ExpenseType) Valid() bool {
	for _, v := range AllExpenseTypes() {
		if e == v {
			return true
		}
	}
	return false
}

// LabelKey is the i18n key of the display name.
func (e ExpenseType) LabelKey() string { return "invoices.expense_type." + string(e) }

// Category is one of the 18 fixed expense categories (FR-9).
type Category string

const (
	CategoryComprasMercadorias      Category = "compras_mercadorias"
	CategoryEquipamentosInformatica Category = "equipamentos_informatica"
	CategoryMaterialEscritorio      Category = "material_escritorio"
	CategoryAgua                    Category = "agua"
	CategoryEletricidade            Category = "eletricidade"
	CategoryGas                     Category = "gas"
	CategoryInternetTelecom         Category = "internet_telecomunicacoes"
	CategoryRendas                  Category = "rendas"
	CategorySeguros                 Category = "seguros"
	CategoryContabilidade           Category = "contabilidade"
	CategoryConsultoria             Category = "consultoria"
	CategoryManutencao              Category = "manutencao"
	CategoryLimpeza                 Category = "limpeza"
	CategoryTransportes             Category = "transportes_deslocacoes"
	CategoryCombustivel             Category = "combustivel"
	CategoryDespesasBancarias       Category = "despesas_bancarias"
	CategoryOutrosServicos          Category = "outros_servicos"
	CategoryOutrasDespesas          Category = "outras_despesas"
)

// AllCategories lists the 18 categories in display order.
func AllCategories() []Category {
	return []Category{
		CategoryComprasMercadorias, CategoryEquipamentosInformatica, CategoryMaterialEscritorio,
		CategoryAgua, CategoryEletricidade, CategoryGas, CategoryInternetTelecom,
		CategoryRendas, CategorySeguros, CategoryContabilidade, CategoryConsultoria,
		CategoryManutencao, CategoryLimpeza, CategoryTransportes, CategoryCombustivel,
		CategoryDespesasBancarias, CategoryOutrosServicos, CategoryOutrasDespesas,
	}
}

// Valid reports whether c is a known value.
func (c Category) Valid() bool {
	for _, v := range AllCategories() {
		if c == v {
			return true
		}
	}
	return false
}

// LabelKey is the i18n key of the display name.
func (c Category) LabelKey() string { return "invoices.category." + string(c) }

// PaymentMethod is how a Pagamento da fatura was made (FR-15).
type PaymentMethod string

const (
	MethodTransferenciaBancaria PaymentMethod = "transferencia_bancaria"
	MethodCartao                PaymentMethod = "cartao"
	MethodDebitoDireto          PaymentMethod = "debito_direto"
	MethodDinheiro              PaymentMethod = "dinheiro"
	MethodMultibanco            PaymentMethod = "multibanco"
	MethodMBWay                 PaymentMethod = "mb_way"
	MethodOutro                 PaymentMethod = "outro"
)

// AllPaymentMethods lists the seven methods.
func AllPaymentMethods() []PaymentMethod {
	return []PaymentMethod{
		MethodTransferenciaBancaria, MethodCartao, MethodDebitoDireto,
		MethodDinheiro, MethodMultibanco, MethodMBWay, MethodOutro,
	}
}

// Valid reports whether m is a known value.
func (m PaymentMethod) Valid() bool {
	for _, v := range AllPaymentMethods() {
		if m == v {
			return true
		}
	}
	return false
}

// LabelKey is the i18n key of the display name.
func (m PaymentMethod) LabelKey() string { return "invoices.payment_method." + string(m) }

// FiscalRegion selects the VAT rate table (FR-12).
type FiscalRegion string

const (
	RegionContinental FiscalRegion = "continental"
	RegionMadeira     FiscalRegion = "madeira"
	RegionAcores      FiscalRegion = "acores"
)

// AllFiscalRegions lists the three regions, continental first (the default).
func AllFiscalRegions() []FiscalRegion {
	return []FiscalRegion{RegionContinental, RegionMadeira, RegionAcores}
}

// Valid reports whether r is a known value.
func (r FiscalRegion) Valid() bool {
	for _, v := range AllFiscalRegions() {
		if r == v {
			return true
		}
	}
	return false
}

// LabelKey is the i18n key of the display name.
func (r FiscalRegion) LabelKey() string { return "invoices.region." + string(r) }

// VATTreatment is how a Linha da fatura is taxed.
type VATTreatment string

const (
	// TreatmentRate charges VAT at a rate allowed for the fiscal region.
	TreatmentRate VATTreatment = "rate"
	// TreatmentExempt charges 0 % and requires an exemption code and reason.
	TreatmentExempt VATTreatment = "exempt"
	// TreatmentReverseCharge charges no VAT (autoliquidação) and is flagged.
	TreatmentReverseCharge VATTreatment = "reverse_charge"
)

// AllVATTreatments lists the three treatments.
func AllVATTreatments() []VATTreatment {
	return []VATTreatment{TreatmentRate, TreatmentExempt, TreatmentReverseCharge}
}

// Valid reports whether t is a known value.
func (t VATTreatment) Valid() bool {
	return t == TreatmentRate || t == TreatmentExempt || t == TreatmentReverseCharge
}

// LabelKey is the i18n key of the display name.
func (t VATTreatment) LabelKey() string { return "invoices.vat_treatment." + string(t) }

// State is the lifecycle of a fatura: Rascunho, registered or Anulação.
type State string

const (
	StateDraft      State = "draft"
	StateRegistered State = "registered"
	StateAnnulled   State = "annulled"
)

// Valid reports whether s is a known value.
func (s State) Valid() bool {
	return s == StateDraft || s == StateRegistered || s == StateAnnulled
}

// PaymentStatus is the derived status shown to the user (FR-16, FR-20, FR-22).
// It is never stored: see Status.
type PaymentStatus string

const (
	StatusPendente         PaymentStatus = "pendente"
	StatusParcialmentePago PaymentStatus = "parcialmente_pago"
	StatusPago             PaymentStatus = "pago"
	StatusAnulado          PaymentStatus = "anulado"
	StatusRascunho         PaymentStatus = "rascunho"
)

// AllPaymentStatuses lists the five statuses.
func AllPaymentStatuses() []PaymentStatus {
	return []PaymentStatus{StatusPendente, StatusParcialmentePago, StatusPago, StatusAnulado, StatusRascunho}
}

// Valid reports whether p is a known value.
func (p PaymentStatus) Valid() bool {
	for _, v := range AllPaymentStatuses() {
		if p == v {
			return true
		}
	}
	return false
}

// LabelKey is the i18n key of the display name.
func (p PaymentStatus) LabelKey() string { return "invoices.status." + string(p) }
