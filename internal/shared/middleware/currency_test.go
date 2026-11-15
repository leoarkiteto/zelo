package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/money"
)

type fakeCurrencyLoader struct {
	cur   money.Currency
	err   error
	calls int
	gotID string
}

func (f *fakeCurrencyLoader) GetCurrency(_ context.Context, id string) (money.Currency, error) {
	f.calls++
	f.gotID = id
	return f.cur, f.err
}

func runWithCurrency(t *testing.T, l *fakeCurrencyLoader, sess *model.Session) money.Currency {
	t.Helper()
	var got money.Currency
	h := WithCurrency(l)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = money.From(r.Context())
	}))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if sess != nil {
		r = r.WithContext(context.WithValue(r.Context(), sessionKey, *sess))
	}
	h.ServeHTTP(httptest.NewRecorder(), r)
	return got
}

func TestWithCurrency(t *testing.T) {
	sess := &model.Session{TokenHash: "tok", UserID: "u1", CondominiumID: "c1"}

	l := &fakeCurrencyLoader{cur: money.EUR}
	if got := runWithCurrency(t, l, sess); got != money.EUR || l.gotID != "c1" {
		t.Fatalf("got %q (id %q), want EUR for c1", got, l.gotID)
	}

	l = &fakeCurrencyLoader{err: errors.New("db down")}
	if got := runWithCurrency(t, l, sess); got != money.BRL {
		t.Fatalf("loader error: got %q, want BRL", got)
	}

	l = &fakeCurrencyLoader{cur: money.EUR}
	if got := runWithCurrency(t, l, nil); got != money.BRL || l.calls != 0 {
		t.Fatalf("anonymous: got %q calls %d, want BRL and no load", got, l.calls)
	}
}
