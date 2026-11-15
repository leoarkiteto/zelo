package domain

import "time"

// Pagination defaults of the invoice list.
const (
	DefaultPageSize = 25
	MaxPageSize     = 100
)

// ListFilter holds the AND-combined list filters (FR-28). Date fields are
// YYYY-MM-DD strings; Status is a PaymentStatus value; empty means no filter.
type ListFilter struct {
	SupplierName, NIF, Number           string
	DocumentType, ExpenseType, Category string
	Status                              string
	IssueFrom, IssueTo, DueFrom, DueTo  string
	ReferencePeriod                     string
	Page, PageSize                      int
}

// Normalized returns the filter with Page >= 1 and 1 <= PageSize <= MaxPageSize
// (DefaultPageSize when unset).
func (f ListFilter) Normalized() ListFilter {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = DefaultPageSize
	}
	if f.PageSize > MaxPageSize {
		f.PageSize = MaxPageSize
	}
	return f
}

// ListRow is one row of the invoice list (FR-27).
type ListRow struct {
	ID, Number, SupplierName, SupplierNIF string
	DocumentType                          DocumentType
	ExpenseType                           ExpenseType
	Category                              Category
	IssueDate, DueDate                    *time.Time
	ReferencePeriod                       string
	TotalCents, PayableCents              int64
	PaidCents, DebtCents                  int64
	State                                 State
	Status                                PaymentStatus
}

// ListTotals sums the registered invoices of the whole filtered set.
type ListTotals struct {
	TotalCents, PayableCents, PaidCents, DebtCents int64
	Count                                          int
}

// ListResult is a page of rows. Totals covers the whole filtered set (not only
// the page) and excludes drafts and annulled invoices. RowCount counts every
// matching row, drafts and annulled included, and drives pagination.
type ListResult struct {
	Rows           []ListRow
	RowCount       int
	Totals         ListTotals
	Page, PageSize int
}
