package services

import (
	"context"
	"errors"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/money"
)

type fakeCondoStore struct {
	currency  money.Currency
	getErr    error
	setErr    error
	setCalled int
}

func (f *fakeCondoStore) GetCurrency(context.Context, string) (money.Currency, error) {
	return f.currency, f.getErr
}

func (f *fakeCondoStore) SetCurrency(_ context.Context, _ string, c money.Currency) error {
	f.setCalled++
	if f.setErr != nil {
		return f.setErr
	}
	f.currency = c
	return nil
}

func TestChangeCurrencyUpdatesAndAudits(t *testing.T) {
	st := &fakeCondoStore{currency: money.BRL}
	au := &fakeAudit{}
	svc := &SettingsService{Condominiums: st, Audit: au}

	if err := svc.ChangeCurrency(context.Background(), "u1", "c1", money.EUR); err != nil {
		t.Fatal(err)
	}
	if st.currency != money.EUR {
		t.Fatalf("currency = %q, want EUR", st.currency)
	}
	if len(au.events) != 1 {
		t.Fatalf("events = %d, want 1", len(au.events))
	}
	ev := au.events[0]
	if ev.EventType != model.AuditCondominiumCurrencyChanged {
		t.Fatalf("event type = %q", ev.EventType)
	}
	if ev.UserID == nil || *ev.UserID != "u1" {
		t.Fatalf("event user = %v, want u1", ev.UserID)
	}
	if ev.Details["condominium_id"] != "c1" || ev.Details["old_currency"] != "BRL" || ev.Details["new_currency"] != "EUR" {
		t.Fatalf("details = %v", ev.Details)
	}
}

func TestChangeCurrencyRejectsInvalid(t *testing.T) {
	st := &fakeCondoStore{currency: money.BRL}
	au := &fakeAudit{}
	svc := &SettingsService{Condominiums: st, Audit: au}

	err := svc.ChangeCurrency(context.Background(), "u1", "c1", money.Currency("USD"))
	if !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("err = %v, want ErrInvalidCurrency", err)
	}
	if st.setCalled != 0 || len(au.events) != 0 {
		t.Fatalf("invalid currency must not write or audit")
	}
}

func TestChangeCurrencyUnchangedIsNoop(t *testing.T) {
	st := &fakeCondoStore{currency: money.EUR}
	au := &fakeAudit{}
	svc := &SettingsService{Condominiums: st, Audit: au}

	if err := svc.ChangeCurrency(context.Background(), "u1", "c1", money.EUR); err != nil {
		t.Fatal(err)
	}
	if st.setCalled != 0 || len(au.events) != 0 {
		t.Fatalf("unchanged currency must not write or audit")
	}
}

func TestChangeCurrencyStoreErrorsSkipAudit(t *testing.T) {
	boom := errors.New("boom")
	au := &fakeAudit{}
	svc := &SettingsService{Condominiums: &fakeCondoStore{currency: money.BRL, setErr: boom}, Audit: au}
	if err := svc.ChangeCurrency(context.Background(), "u1", "c1", money.EUR); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if len(au.events) != 0 {
		t.Fatal("failed write must not audit")
	}

	svc = &SettingsService{Condominiums: &fakeCondoStore{getErr: boom}, Audit: au}
	if err := svc.ChangeCurrency(context.Background(), "u1", "c1", money.EUR); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}
