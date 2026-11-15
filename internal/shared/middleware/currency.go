package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/leoarkiteto/zelo/internal/shared/money"
)

// CurrencyLoader loads a condominium's currency.
type CurrencyLoader interface {
	GetCurrency(ctx context.Context, condoID string) (money.Currency, error)
}

// WithCurrency stores the session condominium's currency in the request
// context (money.From). It must run after WithUser. Anonymous requests, and
// loader failures (logged), fall back to BRL.
func WithCurrency(loader CurrencyLoader) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cur := money.Default()
			if sess := SessionFrom(r.Context()); sess != nil && sess.CondominiumID != "" {
				c, err := loader.GetCurrency(r.Context(), sess.CondominiumID)
				switch {
				case err != nil:
					slog.Default().Error("load condominium currency", "condominium_id", sess.CondominiumID, "error", err)
				case c.Valid():
					cur = c
				}
			}
			next.ServeHTTP(w, r.WithContext(money.WithCurrency(r.Context(), cur)))
		})
	}
}
