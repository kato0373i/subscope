package domain

import (
	"errors"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

type Status string

const (
	StatusDraft  Status = "draft"
	StatusIssued Status = "issued"
	StatusPaid   Status = "paid"
	StatusVoid   Status = "void"
)

// TaxCategory は税区分。billing ドメインが独立して保持し、tax モジュールに依存しない。
type TaxCategory string

const (
	TaxStandard TaxCategory = "standard" // 標準税率 10%
	TaxReduced  TaxCategory = "reduced"  // 軽減税率 8%
	TaxExempt   TaxCategory = "exempt"   // 非課税
)

// InvoiceLine は請求明細。税区分・金額・数量を持つ。
// issued 後は変更不可（修正は CreditNote 別 Invoice で対応）。
type InvoiceLine struct {
	ID          string
	Description string
	NetAmount   shared.Money
	Quantity    int
	TaxCategory TaxCategory
}

// Invoice は債権オブジェクト。決済手段への参照を一切持たないのが設計の核心。
type Invoice struct {
	ID               shared.InvoiceID
	ContractID       shared.ContractID
	BillingAccountID shared.BillingAccountID
	Lines            []InvoiceLine
	Amount           shared.Money // Σ明細の合計（税込）
	Status           Status
}

var ErrAlreadyIssued = errors.New("発行済みの請求書は変更できません")

func NewDraft(id shared.InvoiceID, contract shared.ContractID, account shared.BillingAccountID) *Invoice {
	return &Invoice{
		ID:               id,
		ContractID:       contract,
		BillingAccountID: account,
		Lines:            []InvoiceLine{},
		Status:           StatusDraft,
	}
}

// NewIssued は既存コードとの互換性のため維持する。
func NewIssued(id shared.InvoiceID, contract shared.ContractID, account shared.BillingAccountID, amount shared.Money) *Invoice {
	return &Invoice{
		ID:               id,
		ContractID:       contract,
		BillingAccountID: account,
		Lines:            []InvoiceLine{},
		Amount:           amount,
		Status:           StatusIssued,
	}
}

// AddLine は draft 状態の Invoice に明細を追加する。
func (i *Invoice) AddLine(line InvoiceLine) error {
	if i.Status != StatusDraft {
		return ErrAlreadyIssued
	}
	i.Lines = append(i.Lines, line)
	return nil
}

// Issue は draft → issued へ遷移し、合計金額を確定する。
// issued 後は明細変更不可。
func (i *Invoice) Issue() error {
	if i.Status != StatusDraft {
		return ErrAlreadyIssued
	}
	total := shared.Money{Currency: "JPY"}
	for _, l := range i.Lines {
		amt, err := total.Add(l.NetAmount)
		if err != nil {
			return err
		}
		total = amt
	}
	i.Amount = total
	i.Status = StatusIssued
	return nil
}

// MarkPaid は単調な状態遷移。発行済みからのみ入金済みへ進める。
func (i *Invoice) MarkPaid() {
	if i.Status == StatusIssued {
		i.Status = StatusPaid
	}
}

// MarkVoid は発行済みの請求書を無効化する（修正は赤伝＝CreditNote で対応）。
// issued からのみ void へ進める。
func (i *Invoice) MarkVoid() {
	if i.Status == StatusIssued {
		i.Status = StatusVoid
	}
}
