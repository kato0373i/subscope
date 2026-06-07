package domain

import (
	"errors"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

var (
	// ErrOverApplication は入金額を超える充当（過充当）を弾く。
	ErrOverApplication = errors.New("入金額を超える充当はできません")
	// ErrCurrencyMismatch は通貨不一致の充当を弾く。
	ErrCurrencyMismatch = errors.New("通貨が一致しません")
	// ErrInvalidAllocationAmount は 0 円以下の充当を弾く（applied の減算による不正状態を防ぐ）。
	ErrInvalidAllocationAmount = errors.New("充当額は 1 円以上である必要があります")
)

// Allocation は 1 入金の一部を 1 請求へ充当した記録。
type Allocation struct {
	Invoice shared.InvoiceID
	Amount  shared.Money
}

// BankDeposit は銀行口座への 1 入金。1 入金を複数請求へ按分して充当できる（団体一括の戻し込み）。
// 不変条件：Σ充当額 ≤ 入金額（過充当の禁止）。
type BankDeposit struct {
	ID          shared.SettlementID
	Reference   string
	Account     shared.BillingAccountID
	PayerName   string
	Amount      shared.Money
	applied     shared.Money
	allocations []Allocation
}

// NewBankDeposit は取り込んだ入金 1 件を生成する。
func NewBankDeposit(id shared.SettlementID, ref string, account shared.BillingAccountID, payer string, amount shared.Money) *BankDeposit {
	return &BankDeposit{
		ID:        id,
		Reference: ref,
		Account:   account,
		PayerName: payer,
		Amount:    amount,
		applied:   shared.Money{Currency: amount.Currency},
	}
}

// Allocate は入金の一部（または全部）を 1 請求へ充当する。過充当・通貨不一致は弾く。
func (d *BankDeposit) Allocate(invoice shared.InvoiceID, amount shared.Money) error {
	if amount.Amount <= 0 {
		return ErrInvalidAllocationAmount
	}
	if amount.Currency != d.Amount.Currency {
		return ErrCurrencyMismatch
	}
	next, err := d.applied.Add(amount)
	if err != nil {
		return err
	}
	if next.Amount > d.Amount.Amount {
		return ErrOverApplication
	}
	d.applied = next
	d.allocations = append(d.allocations, Allocation{Invoice: invoice, Amount: amount})
	return nil
}

// Remaining は未充当の残額。
func (d *BankDeposit) Remaining() shared.Money {
	return shared.Money{Amount: d.Amount.Amount - d.applied.Amount, Currency: d.Amount.Currency}
}

// Applied は充当済みの累計額。
func (d *BankDeposit) Applied() shared.Money { return d.applied }

// Allocations は充当の明細。
func (d *BankDeposit) Allocations() []Allocation { return d.allocations }

// FullyApplied は入金の全額が充当済みかどうか。
func (d *BankDeposit) FullyApplied() bool { return d.applied.Amount == d.Amount.Amount }
