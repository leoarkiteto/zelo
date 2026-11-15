package store

import (
	"context"
	"errors"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/money"
)

func TestCondominiumStoreCurrency(t *testing.T) {
	db := openStoreTestDB(t)
	ctx := context.Background()
	var id string
	if err := db.QueryRowContext(ctx, `INSERT INTO condominiums (name) VALUES ('Currency Test') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("insert condominium: %v", err)
	}
	cs := NewCondominiumStore(db)

	got, err := cs.GetCurrency(ctx, id)
	if err != nil || got != money.BRL {
		t.Fatalf("default currency = %q, %v; want BRL", got, err)
	}
	if err := cs.SetCurrency(ctx, id, money.EUR); err != nil {
		t.Fatalf("set currency: %v", err)
	}
	got, err = cs.GetCurrency(ctx, id)
	if err != nil || got != money.EUR {
		t.Fatalf("currency = %q, %v; want EUR", got, err)
	}
	if err := cs.SetCurrency(ctx, id, money.Currency("USD")); err == nil {
		t.Fatal("expected error for invalid currency")
	}
	missing := "00000000-0000-0000-0000-000000000000"
	if _, err := cs.GetCurrency(ctx, missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get missing = %v, want ErrNotFound", err)
	}
	if err := cs.SetCurrency(ctx, missing, money.EUR); !errors.Is(err, ErrNotFound) {
		t.Fatalf("set missing = %v, want ErrNotFound", err)
	}
}
