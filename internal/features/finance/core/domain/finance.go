// Package domain contains finance-specific domain rules: account types,
// statuses, category keys, money parsing, and status transitions.
package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Domain validation errors returned by the finance package.
var (
	// ErrInvalidAmount is returned when an amount is malformed, zero, or negative.
	ErrInvalidAmount = errors.New("invalid amount")
	// ErrInvalidTitle is returned when the title is empty or too long.
	ErrInvalidTitle = errors.New("invalid title")
	// ErrInvalidCategory is returned when the category is not valid for the type.
	ErrInvalidCategory = errors.New("invalid category")
	// ErrInvalidDueDate is returned when the due date is malformed.
	ErrInvalidDueDate = errors.New("invalid due date")
	// ErrInvalidSettlementDate is returned when the settlement date is malformed.
	ErrInvalidSettlementDate = errors.New("invalid settlement date")
	// ErrSettlementDateRequired is returned when settling without a date.
	ErrSettlementDateRequired = errors.New("settlement date required")
	// ErrInvalidStatus is returned when a status is not a valid stored status.
	ErrInvalidStatus = errors.New("invalid status")
	// ErrNotEditable is returned when editing a settled or canceled account.
	ErrNotEditable = errors.New("account is not editable")
	// ErrNotSettlable is returned when settling a settled or canceled account.
	ErrNotSettlable = errors.New("account is not settlable")
	// ErrNotCancellable is returned when canceling a settled or canceled account.
	ErrNotCancellable = errors.New("account is not cancellable")
)

// AccountType is the kind of financial account.
type AccountType string

const (
	// AccountTypePayable is an expense (conta a pagar).
	AccountTypePayable AccountType = "payable"
	// AccountTypeReceivable is revenue (conta a receber).
	AccountTypeReceivable AccountType = "receivable"
)

// Valid reports whether t is a supported account type.
func (t AccountType) Valid() bool {
	return t == AccountTypePayable || t == AccountTypeReceivable
}

// AccountStatus is the persisted lifecycle status. Overdue is derived and
// never persisted (research.md R3).
type AccountStatus string

const (
	// StatusPending is the initial status for most accounts.
	StatusPending AccountStatus = "pending"
	// StatusSettled means paid/received (Liquidado).
	StatusSettled AccountStatus = "settled"
	// StatusCanceled means canceled (Cancelado).
	StatusCanceled AccountStatus = "canceled"
	// StatusOverdue is derived: pending with due_date before today.
	StatusOverdue AccountStatus = "overdue"
)

// ValidStored reports whether s is a status that may be persisted.
func (s AccountStatus) ValidStored() bool {
	return s == StatusPending || s == StatusSettled || s == StatusCanceled
}

// Category is a stable category key; display labels live in the i18n catalog.
type Category string

// Payable categories (spec FR-003).
const (
	CategoryMaintenance        Category = "maintenance"
	CategoryCleaning           Category = "cleaning"
	CategoryUtilities          Category = "utilities"
	CategoryPayroll            Category = "payroll"
	CategoryThirdPartyServices Category = "third_party_services"
	CategoryOther              Category = "other"
)

// Receivable categories (spec FR-006).
const (
	CategoryCondoFee             Category = "condo_fee"
	CategoryFineInterest         Category = "fine_interest"
	CategoryCommonAreaReservation Category = "common_area_reservation"
	CategoryExtraordinaryIncome  Category = "extraordinary_income"
)

// ValidFor reports whether c is a valid category for the account type.
func (c Category) ValidFor(t AccountType) bool {
	switch t {
	case AccountTypePayable:
		switch c {
		case CategoryMaintenance, CategoryCleaning, CategoryUtilities,
			CategoryPayroll, CategoryThirdPartyServices, CategoryOther:
			return true
		}
	case AccountTypeReceivable:
		switch c {
		case CategoryCondoFee, CategoryFineInterest,
			CategoryCommonAreaReservation, CategoryExtraordinaryIncome:
			return true
		}
	}
	return false
}

// PayableCategories returns the supported payable category keys in display order.
func PayableCategories() []Category {
	return []Category{CategoryMaintenance, CategoryCleaning, CategoryUtilities,
		CategoryPayroll, CategoryThirdPartyServices, CategoryOther}
}

// ReceivableCategories returns the supported receivable category keys in display order.
func ReceivableCategories() []Category {
	return []Category{CategoryCondoFee, CategoryFineInterest,
		CategoryCommonAreaReservation, CategoryExtraordinaryIncome}
}

// CategoriesFor returns the category keys valid for the account type.
func CategoriesFor(t AccountType) []Category {
	if t == AccountTypePayable {
		return PayableCategories()
	}
	return ReceivableCategories()
}

// FinancialAccount is a persisted account to pay or to receive.
type FinancialAccount struct {
	ID             string
	CondominiumID  string
	Type           AccountType
	Title          string
	Category       Category
	AmountCents    int64
	DueDate        time.Time
	Status         AccountStatus
	SettlementDate *time.Time
	SupplierPayee  string
	ReceiptPath    string
	ReceiptName    string
	UnitID         string
	PaymentCode    string
	CreatedBy      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// EffectiveStatus returns overdue when the account is pending and its due date
// is strictly before today (spec FR-009). A due date equal to today stays pending.
func (a FinancialAccount) EffectiveStatus(today time.Time) AccountStatus {
	if a.Status == StatusPending && a.DueDate.Before(today) {
		return StatusOverdue
	}
	return a.Status
}

// CanEdit reports whether the account can be edited or transitioned. Overdue
// accounts have StatusPending, so one check covers both (FR-019).
func (a FinancialAccount) CanEdit() bool { return a.Status == StatusPending }

// CanSettle reports whether the account can be settled.
func (a FinancialAccount) CanSettle() bool { return a.Status == StatusPending }

// CanCancel reports whether the account can be canceled.
func (a FinancialAccount) CanCancel() bool { return a.Status == StatusPending }

const (
	maxTitleLen      = 120
	maxSupplierLen   = 120
	maxPaymentCodeLen = 200
	// dateLayout is the accepted form input format.
	dateLayout = "2006-01-02"
)

// ValidateTitle checks the account title.
func ValidateTitle(s string) error {
	s = strings.TrimSpace(s)
	if s == "" || len([]rune(s)) > maxTitleLen {
		return ErrInvalidTitle
	}
	return nil
}

// ValidateSupplier checks the optional supplier/payee field.
func ValidateSupplier(s string) error {
	if s == "" {
		return nil
	}
	if len([]rune(s)) > maxSupplierLen {
		return ErrInvalidTitle
	}
	return nil
}

// ValidatePaymentCode checks the optional payment code/line field.
func ValidatePaymentCode(s string) error {
	if s == "" {
		return nil
	}
	if len([]rune(s)) > maxPaymentCodeLen {
		return ErrInvalidTitle
	}
	return nil
}

// ParseAmountCents converts a decimal string ("1234.56") to integer cents.
// Values with more than two decimal places, zero, or negative values are
// rejected (FR-011).
func ParseAmountCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, ",") {
		return 0, ErrInvalidAmount
	}
	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(strings.TrimPrefix(s, "+"), "-")

	wholePart := s
	fracPart := ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		wholePart, fracPart = s[:i], s[i+1:]
		if wholePart == "" || fracPart == "" {
			return 0, ErrInvalidAmount
		}
		if len(fracPart) > 2 {
			return 0, ErrInvalidAmount
		}
		for len(fracPart) < 2 {
			fracPart += "0"
		}
	} else if s == "" {
		return 0, ErrInvalidAmount
	}

	whole, err := strconv.ParseInt(wholePart, 10, 64)
	if err != nil {
		return 0, ErrInvalidAmount
	}
	frac := int64(0)
	if fracPart != "" {
		frac, err = strconv.ParseInt(fracPart, 10, 64)
		if err != nil {
			return 0, ErrInvalidAmount
		}
	}
	cents := whole*100 + frac
	if negative {
		cents = -cents
	}
	if cents <= 0 {
		return 0, ErrInvalidAmount
	}
	return cents, nil
}

// ParseDate parses a form date in YYYY-MM-DD.
func ParseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, ErrInvalidDueDate
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, ErrInvalidDueDate
	}
	return t, nil
}

// ParseSettlementDate parses a settlement date, distinguishing the
// "required but missing" case from a malformed date.
func ParseSettlementDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, ErrSettlementDateRequired
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %v", ErrInvalidSettlementDate, err)
	}
	return t, nil
}
