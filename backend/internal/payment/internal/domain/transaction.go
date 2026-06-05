package domain

import "github.com/kato0373i/subscope/backend/internal/shared"

type Status string

const (
	StatusRequested Status = "requested"
	StatusCaptured  Status = "captured"
	StatusFailed    Status = "failed"
	StatusPending   Status = "pending" // 口座振替・払込票は結果が後日確定する
)

// Transaction は 1 回の決済試行。invoice_id と payment_method_id がここで初めて出会う。
// 1 試行は 1 手段・1 金額に固定し、手段を変える場合は新しい Transaction を作る。
type Transaction struct {
	ID            shared.TransactionID
	Invoice       shared.InvoiceID
	PaymentMethod shared.PaymentMethodID
	Amount        shared.Money
	Status        Status
	FailureReason string
}

func NewTransaction(id shared.TransactionID, invoice shared.InvoiceID, method shared.PaymentMethodID, amount shared.Money) *Transaction {
	return &Transaction{
		ID:            id,
		Invoice:       invoice,
		PaymentMethod: method,
		Amount:        amount,
		Status:        StatusRequested,
	}
}

func (t *Transaction) MarkCaptured()            { t.Status = StatusCaptured }
func (t *Transaction) MarkFailed(reason string) { t.Status = StatusFailed; t.FailureReason = reason }
