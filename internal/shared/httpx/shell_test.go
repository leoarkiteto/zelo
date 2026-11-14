package httpx

import (
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

func TestNavForHasNoFinanceHealthItem(t *testing.T) {
	for _, roles := range [][]model.Role{{model.RoleOwner}, {model.RoleSyndic}} {
		for _, it := range NavFor(roles, "/", i18n.LanguageEN) {
			if it.Path == "/finance/health" {
				t.Fatalf("nav for %v still contains /finance/health", roles)
			}
		}
	}
}
