package domain

// Status derives the payment status of a fatura. It is never stored. The
// repository list query mirrors these exact rules in a SQL CASE; keep both in
// sync (and see the repository parity test).
//
// Rules, evaluated in this order (paid is the sum of the payments):
//
//	state = draft                      -> rascunho
//	state = annulled                   -> anulado
//	paid >= payable                    -> pago
//	paid > 0 (and paid < payable)      -> parcialmente_pago
//	otherwise (paid = 0 < payable)     -> pendente
//
// So a registered invoice with payable 0 (retenção na fonte equal to the
// total) is pago even with no payments (FR-16), and an overpaid invoice is
// pago (the surplus is a crédito a favor, see DebtCents).
func Status(state State, payableCents, paidCents int64) PaymentStatus {
	switch {
	case state == StateDraft:
		return StatusRascunho
	case state == StateAnnulled:
		return StatusAnulado
	case paidCents >= payableCents:
		return StatusPago
	case paidCents > 0:
		return StatusParcialmentePago
	default:
		return StatusPendente
	}
}

// DebtCents is the Valor em dívida: payable − paid. A negative result is a
// crédito a favor (see CreditCents).
func DebtCents(payableCents, paidCents int64) int64 { return payableCents - paidCents }

// CreditCents is the crédito a favor: the amount paid above the valor a
// pagar, or 0.
func CreditCents(payableCents, paidCents int64) int64 {
	if d := DebtCents(payableCents, paidCents); d < 0 {
		return -d
	}
	return 0
}
