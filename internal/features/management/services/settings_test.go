package services

import (
	"context"
	"errors"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/money"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

type fakeCondoStore struct {
	currency  money.Currency
	lateFee   model.LateFeeSettings
	getErr    error
	setErr    error
	setCalled int
	lateSets  int
}

func (f *fakeCondoStore) GetCurrency(context.Context, string) (money.Currency, error) {
	return f.currency, f.getErr
}

func (f *fakeCondoStore) GetLateFeeSettings(context.Context, string) (model.LateFeeSettings, error) {
	return f.lateFee, f.getErr
}

func (f *fakeCondoStore) SetLateFeeSettings(_ context.Context, _ string, v model.LateFeeSettings, maxBRLFeeBP int) error {
	if f.currency == money.BRL && v.FeeBP > maxBRLFeeBP {
		return store.ErrLateFeeAboveBRLCap
	}
	f.lateSets++
	f.lateFee = v
	return nil
}

func (f *fakeCondoStore) SetCurrencyClampingLateFee(_ context.Context, _ string, c money.Currency, maxBP int) error {
	f.setCalled++
	if f.setErr != nil {
		return f.setErr
	}
	f.currency = c
	if c == money.BRL && f.lateFee.FeeBP > maxBP {
		f.lateFee.FeeBP = maxBP
	}
	return nil
}

func TestChangeCurrencyToBRLClampsAndAudits(t *testing.T) {
	st := &fakeCondoStore{currency: money.EUR, lateFee: model.LateFeeSettings{FeeBP: 500, InterestBP: 100}}
	au := &fakeAudit{}
	svc := &SettingsService{Condominiums: st, Audit: au}
	if err := svc.ChangeCurrency(context.Background(), "u1", "c1", money.BRL); err != nil {
		t.Fatal(err)
	}
	if st.lateFee.FeeBP != 200 || st.setCalled != 1 {
		t.Fatalf("fee = %d, calls = %d", st.lateFee.FeeBP, st.setCalled)
	}
	if len(au.events) != 2 || au.events[1].EventType != model.AuditLateFeeSettingsChanged {
		t.Fatalf("events = %+v", au.events)
	}
	if au.events[1].Details["old_fee_bp"] != 500 || au.events[1].Details["new_fee_bp"] != 200 {
		t.Fatalf("details = %v", au.events[1].Details)
	}
}

func TestChangeCurrencyToBRLWithoutClampAuditsOnlyCurrency(t *testing.T) {
	st := &fakeCondoStore{currency: money.EUR, lateFee: model.LateFeeSettings{FeeBP: 200}}
	au := &fakeAudit{}
	svc := &SettingsService{Condominiums: st, Audit: au}
	if err := svc.ChangeCurrency(context.Background(), "u1", "c1", money.BRL); err != nil {
		t.Fatal(err)
	}
	if len(au.events) != 1 {
		t.Fatalf("events = %d, want 1", len(au.events))
	}
}

func TestSaveLateFeeBRLCap(t *testing.T) {
	st := &fakeCondoStore{currency: money.BRL, lateFee: model.LateFeeSettings{FeeBP: 200, InterestBP: 100}}
	au := &fakeAudit{}
	svc := &SettingsService{Condominiums: st, Audit: au}

	err := svc.SaveLateFee(context.Background(), "u1", "c1", LateFeeInput{Fee: "2,5", Interest: "1", GraceDays: "0"})
	if !errors.Is(err, ErrFeeAboveBRLCap) || st.lateSets != 0 || len(au.events) != 0 {
		t.Fatalf("err = %v, sets = %d, events = %d", err, st.lateSets, len(au.events))
	}

	err = svc.SaveLateFee(context.Background(), "u1", "c1", LateFeeInput{Fee: "2", Interest: "1.5", GraceDays: "5"})
	if err != nil {
		t.Fatal(err)
	}
	want := model.LateFeeSettings{FeeBP: 200, InterestBP: 150, GraceDays: 5}
	if st.lateFee != want {
		t.Fatalf("stored = %+v", st.lateFee)
	}
	if len(au.events) != 1 || au.events[0].EventType != model.AuditLateFeeSettingsChanged {
		t.Fatalf("events = %+v", au.events)
	}
}

func TestSaveLateFeeEURAcceptsFivePercent(t *testing.T) {
	st := &fakeCondoStore{currency: money.EUR}
	svc := &SettingsService{Condominiums: st, Audit: &fakeAudit{}}
	if err := svc.SaveLateFee(context.Background(), "u1", "c1", LateFeeInput{Fee: "5", Interest: "0", GraceDays: "0"}); err != nil {
		t.Fatal(err)
	}
	if st.lateFee.FeeBP != 500 {
		t.Fatalf("fee = %d", st.lateFee.FeeBP)
	}
}

func TestSaveLateFeeValidation(t *testing.T) {
	st := &fakeCondoStore{currency: money.EUR}
	svc := &SettingsService{Condominiums: st}
	cases := []struct {
		in   LateFeeInput
		want error
	}{
		{LateFeeInput{"100.01", "0", "0"}, ErrInvalidFee},
		{LateFeeInput{"1.234", "0", "0"}, ErrInvalidFee},
		{LateFeeInput{"abc", "0", "0"}, ErrInvalidFee},
		{LateFeeInput{"1", "-1", "0"}, ErrInvalidInterest},
		{LateFeeInput{"1", "", "0"}, ErrInvalidInterest},
		{LateFeeInput{"1", "1", "61"}, ErrInvalidGrace},
		{LateFeeInput{"1", "1", "-1"}, ErrInvalidGrace},
		{LateFeeInput{"1", "1", "x"}, ErrInvalidGrace},
	}
	for _, c := range cases {
		if err := svc.SaveLateFee(context.Background(), "u1", "c1", c.in); !errors.Is(err, c.want) {
			t.Errorf("%+v: err = %v, want %v", c.in, err, c.want)
		}
	}
	if st.lateSets != 0 {
		t.Fatal("invalid input must not write")
	}
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
