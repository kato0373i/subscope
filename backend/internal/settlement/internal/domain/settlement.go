package domain

import "github.com/kato0373i/subscope/backend/internal/shared"

// Settlement は実際に入金された事実と、それを債権へ適用した消込。
type Settlement struct {
	ID      shared.SettlementID
	Invoice shared.InvoiceID
	Amount  shared.Money
}

func New(id shared.SettlementID, invoice shared.InvoiceID, amount shared.Money) *Settlement {
	return &Settlement{ID: id, Invoice: invoice, Amount: amount}
}
