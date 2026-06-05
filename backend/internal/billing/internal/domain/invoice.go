package domain

import "github.com/kato0373i/subscope/backend/internal/shared"

type Status string

const (
	StatusIssued Status = "issued"
	StatusPaid   Status = "paid"
	StatusVoid   Status = "void"
)

// Invoice は債権オブジェクト。決済手段への参照を一切持たないのが設計の核心。
type Invoice struct {
	ID               shared.InvoiceID
	ContractID       shared.ContractID
	BillingAccountID shared.BillingAccountID
	Amount           shared.Money
	Status           Status
}

func NewIssued(id shared.InvoiceID, contract shared.ContractID, account shared.BillingAccountID, amount shared.Money) *Invoice {
	return &Invoice{
		ID:               id,
		ContractID:       contract,
		BillingAccountID: account,
		Amount:           amount,
		Status:           StatusIssued,
	}
}

// MarkPaid は単調な状態遷移。発行済みからのみ入金済みへ進める。
func (i *Invoice) MarkPaid() {
	if i.Status == StatusIssued {
		i.Status = StatusPaid
	}
}
