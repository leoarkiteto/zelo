package domain

import "time"

// Supplier is a Fornecedor of one condominium (FR-5).
type Supplier struct {
	ID            string
	CondominiumID string
	Name          string
	NIF           string // normalized with NormalizeNIF
	Country       string // ISO code, default PT
	Address       string
	PostalCode    string
	Locality      string
	Email         string
	Phone         string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Line is one Linha da fatura. For compra de produtos (a product line) the
// base is computed from QuantityMilli, UnitPriceCents and DiscountCents; for
// the other expense types BaseCents is the amount given by the user. See
// IsProductLine.
type Line struct {
	Description, Product, Reference, Unit string
	QuantityMilli                         int64 // 2.5 units = 2500
	UnitPriceCents                        int64
	DiscountCents                         int64
	BaseCents                             int64
	Treatment                             VATTreatment
	RateBP                                int // basis points; meaningful for TreatmentRate only
	ExemptionCode, ExemptionReason        string
}

// TaxBucket is one row of the per-rate VAT breakdown (FR-13). Exempt and
// reverse-charge lines form their own buckets with RateBP 0.
type TaxBucket struct {
	Treatment VATTreatment
	RateBP    int
	BaseCents int64
	VATCents  int64
}

// Totals are always computed from the lines by ComputeTotals (FR-13, FR-14).
type Totals struct {
	GrossCents       int64 // sum of the lines before discount
	DiscountCents    int64 // sum of the line discounts
	BaseCents        int64 // sum of the bases (gross - discounts)
	VATCents         int64
	TotalCents       int64 // bases + VAT
	WithholdingCents int64 // Retenção na fonte, entered by the user
	PayableCents     int64 // Valor a pagar = total - withholding
	Taxes            []TaxBucket
}

// ServiceDetails are the optional fields of a prestação de serviços (FR-10).
// The project is a common field of Invoice.
type ServiceDetails struct {
	Description string
	StartDate   *time.Time
	EndDate     *time.Time
	Contract    string
}

// MonthlyDetails are the optional fields of a despesa mensal (FR-10).
type MonthlyDetails struct {
	ServiceType      string
	BillingPeriod    string
	StartDate        *time.Time
	EndDate          *time.Time
	ContractNumber   string
	MeterID          string
	CPE              string
	CUI              string
	ConsumptionPlace string
	Recurring        bool
}

// Payment is one Pagamento da fatura (FR-15). Justification is set when it
// exceeded the valor a pagar (FR-17).
type Payment struct {
	ID            string
	InvoiceID     string
	Date          time.Time
	AmountCents   int64
	Method        PaymentMethod
	Reference     string
	BankAccount   string
	Justification string
	CreatedAt     time.Time
	CreatedBy     string
}

// Attachment is a stored file of an invoice (FR-25).
type Attachment struct {
	ID           string
	InvoiceID    string
	OriginalName string
	StoredPath   string
	ContentType  string
	SizeBytes    int64
	CreatedAt    time.Time
	CreatedBy    string
}

// Invoice is a Fatura received from a Fornecedor. Totals, Taxes and Lines are
// derived; persistence writes the header totals only from ComputeTotals.
type Invoice struct {
	ID             string
	CondominiumID  string
	State          State
	IdempotencyKey string

	DocumentType  DocumentType
	ExpenseType   ExpenseType
	Number        string // normalized with NormalizeNumber
	IssueDate     *time.Time
	DueDate       *time.Time
	OperationDate *time.Time
	SupplierID    string
	Supplier      *Supplier // loaded for views
	Description   string
	// DueJustification is stored when the due date precedes the issue date (FR-18).
	DueJustification string

	Category        Category
	Project         string
	CostCenter      string
	ReferencePeriod string // YYYY-MM
	Notes           string

	FiscalRegion FiscalRegion
	Service      ServiceDetails
	Monthly      MonthlyDetails
	Lines        []Line
	Totals       Totals

	// OriginalInvoiceID is reserved for credit notes (FR-34), unused in phase 1.
	OriginalInvoiceID string

	Payments    []Payment
	Attachments []Attachment

	AnnulReason string
	AnnulledAt  *time.Time
	DeletedAt   *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

// PaidCents sums the payments of the invoice.
func (i Invoice) PaidCents() int64 {
	var sum int64
	for _, p := range i.Payments {
		sum += p.AmountCents
	}
	return sum
}
