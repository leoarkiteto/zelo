package domain

import (
	"errors"
	"testing"
	"time"
)

func TestParseAmountCents(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    int64
		wantErr error
	}{
		{"integer", "350", 35000, nil},
		{"two decimals", "1234.56", 123456, nil},
		{"one decimal padded", "10.5", 1050, nil},
		{"zero", "0", 0, ErrInvalidAmount},
		{"zero with cents", "0.00", 0, ErrInvalidAmount},
		{"negative", "-10", 0, ErrInvalidAmount},
		{"negative decimal", "-1.50", 0, ErrInvalidAmount},
		{"three decimals", "1.234", 0, ErrInvalidAmount},
		{"empty", "", 0, ErrInvalidAmount},
		{"comma decimal", "1,50", 0, ErrInvalidAmount},
		{"non numeric", "abc", 0, ErrInvalidAmount},
		{"missing fraction", "12.", 0, ErrInvalidAmount},
		{"missing whole", ".50", 0, ErrInvalidAmount},
		{"whitespace", "  42  ", 4200, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAmountCents(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ParseAmountCents(%q) error = %v, want %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("ParseAmountCents(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestCategoryValidFor(t *testing.T) {
	if !CategoryMaintenance.ValidFor(AccountTypePayable) {
		t.Error("maintenance should be valid for payable")
	}
	if !CategoryCondoFee.ValidFor(AccountTypeReceivable) {
		t.Error("condo_fee should be valid for receivable")
	}
	if CategoryCondoFee.ValidFor(AccountTypePayable) {
		t.Error("condo_fee must not be valid for payable")
	}
	if CategoryMaintenance.ValidFor(AccountTypeReceivable) {
		t.Error("maintenance must not be valid for receivable")
	}
	if Category("bogus").ValidFor(AccountTypePayable) {
		t.Error("bogus category must be rejected")
	}
}

func TestCategoriesFor(t *testing.T) {
	pay := CategoriesFor(AccountTypePayable)
	if len(pay) != 6 {
		t.Fatalf("payable categories = %d, want 6", len(pay))
	}
	rec := CategoriesFor(AccountTypeReceivable)
	if len(rec) != 4 {
		t.Fatalf("receivable categories = %d, want 4", len(rec))
	}
}

func TestEffectiveStatus(t *testing.T) {
	today := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	past := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	accounts := []struct {
		name string
		acc  FinancialAccount
		want AccountStatus
	}{
		{"pending future stays pending", FinancialAccount{Status: StatusPending, DueDate: time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)}, StatusPending},
		{"pending today stays pending", FinancialAccount{Status: StatusPending, DueDate: today}, StatusPending},
		{"pending past becomes overdue", FinancialAccount{Status: StatusPending, DueDate: past}, StatusOverdue},
		{"settled past stays settled", FinancialAccount{Status: StatusSettled, DueDate: past}, StatusSettled},
		{"canceled past stays canceled", FinancialAccount{Status: StatusCanceled, DueDate: past}, StatusCanceled},
	}
	for _, tt := range accounts {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.acc.EffectiveStatus(today); got != tt.want {
				t.Fatalf("EffectiveStatus = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTransitionGuards(t *testing.T) {
	pending := FinancialAccount{Status: StatusPending}
	settled := FinancialAccount{Status: StatusSettled}
	canceled := FinancialAccount{Status: StatusCanceled}

	if !pending.CanEdit() || !pending.CanSettle() || !pending.CanCancel() {
		t.Error("pending account must be editable, settlable, and cancellable")
	}
	if settled.CanEdit() || settled.CanSettle() || settled.CanCancel() {
		t.Error("settled account must be read-only")
	}
	if canceled.CanEdit() || canceled.CanSettle() || canceled.CanCancel() {
		t.Error("canceled account must be read-only")
	}
}

func TestValidateTitle(t *testing.T) {
	if err := ValidateTitle(""); err != ErrInvalidTitle {
		t.Errorf("empty title error = %v, want ErrInvalidTitle", err)
	}
	if err := ValidateTitle("  "); err != ErrInvalidTitle {
		t.Errorf("blank title error = %v, want ErrInvalidTitle", err)
	}
	if err := ValidateTitle("Manutenção do portão"); err != nil {
		t.Errorf("valid title rejected: %v", err)
	}
}

func TestParseDates(t *testing.T) {
	d, err := ParseDate("2026-08-23")
	if err != nil {
		t.Fatalf("ParseDate valid: %v", err)
	}
	if d.Format("2006-01-02") != "2026-08-23" {
		t.Fatalf("ParseDate = %v", d)
	}
	if _, err := ParseDate("23/08/2026"); !errors.Is(err, ErrInvalidDueDate) {
		t.Errorf("bad date error = %v, want ErrInvalidDueDate", err)
	}
	if _, err := ParseDate(""); !errors.Is(err, ErrInvalidDueDate) {
		t.Errorf("empty date error = %v, want ErrInvalidDueDate", err)
	}
	if _, err := ParseSettlementDate(""); !errors.Is(err, ErrSettlementDateRequired) {
		t.Errorf("empty settlement error = %v, want ErrSettlementDateRequired", err)
	}
	if _, err := ParseSettlementDate("2026-08-23"); err != nil {
		t.Errorf("valid settlement date rejected: %v", err)
	}
}
