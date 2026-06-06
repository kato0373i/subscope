package domain

import (
	"errors"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

// ErrOverReconciliation は入金額を超える消込（過消込）を弾く。
var ErrOverReconciliation = errors.New("消込額の合計が入金額を超えています")

// Settlement は実際に入金された事実と、それを債権へ適用した消込。
// Amount は入金額（received）。reconciled は債権へ充当済みの累計で、
// 「Σ消込額 ≤ 入金額」を不変条件として保つ（過消込禁止）。
type Settlement struct {
	ID         shared.SettlementID
	Invoice    shared.InvoiceID
	Amount     shared.Money
	reconciled shared.Money
}

func New(id shared.SettlementID, invoice shared.InvoiceID, amount shared.Money) *Settlement {
	return &Settlement{
		ID:         id,
		Invoice:    invoice,
		Amount:     amount,
		reconciled: shared.Money{Currency: amount.Currency},
	}
}

// Reconcile は入金を債権へ充当する。過消込（Σ消込 > 入金額）になる場合は弾く。
func (s *Settlement) Reconcile(amount shared.Money) error {
	next, err := s.reconciled.Add(amount)
	if err != nil {
		return err
	}
	if next.Amount > s.Amount.Amount {
		return ErrOverReconciliation
	}
	s.reconciled = next
	return nil
}

// Reconciled は債権へ充当済みの累計額。
func (s *Settlement) Reconciled() shared.Money { return s.reconciled }

// FullyReconciled は入金額の全額が充当済みかどうか。
func (s *Settlement) FullyReconciled() bool { return s.reconciled.Amount == s.Amount.Amount }
