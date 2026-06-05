// Package events はモジュール間の統合イベント（モジュール間の契約）を定義する。
// 各モジュールの「ドメイン内イベント」はモジュール private に閉じ、ここには出さない。
package events

import "github.com/kato0373i/subscope/backend/internal/shared"

const (
	NameBillingDue       = "contract.BillingDue"
	NameInvoiceIssued    = "billing.InvoiceIssued"
	NameChargeRequested  = "collection.ChargeRequested"
	NamePaymentSucceeded = "payment.PaymentSucceeded"
	NamePaymentFailed    = "payment.PaymentFailed"
	NameInvoicePaid      = "settlement.InvoicePaid"
)

// BillingDue は契約の請求サイクル到来。contract が発行する。
type BillingDue struct {
	ContractID       shared.ContractID
	BillingAccountID shared.BillingAccountID
	Amount           shared.Money
	Period           string
}

func (BillingDue) EventName() string { return NameBillingDue }

// InvoiceIssued は請求書（債権）の発行。billing が発行する。
// 「いくら回収すべきか」だけを伝え、決済手段の情報は一切含まない。
type InvoiceIssued struct {
	InvoiceID        shared.InvoiceID
	BillingAccountID shared.BillingAccountID
	Amount           shared.Money
}

func (InvoiceIssued) EventName() string { return NameInvoiceIssued }

// ChargeRequested は回収戦略が選んだ決済手段での課金要求。collection が発行する。
// ここで初めて InvoiceID と PaymentMethodID が同居する。
type ChargeRequested struct {
	InvoiceID       shared.InvoiceID
	PaymentMethodID shared.PaymentMethodID
	Amount          shared.Money
}

func (ChargeRequested) EventName() string { return NameChargeRequested }

// PaymentSucceeded は決済試行の成功。payment が発行する。
type PaymentSucceeded struct {
	InvoiceID     shared.InvoiceID
	TransactionID shared.TransactionID
	Amount        shared.Money
}

func (PaymentSucceeded) EventName() string { return NamePaymentSucceeded }

// PaymentFailed は決済試行の失敗。payment が発行し、collection が手段の切替を判断する。
type PaymentFailed struct {
	InvoiceID       shared.InvoiceID
	TransactionID   shared.TransactionID
	PaymentMethodID shared.PaymentMethodID
	Reason          string
}

func (PaymentFailed) EventName() string { return NamePaymentFailed }

// InvoicePaid は入金の消込完了。settlement が発行する。
type InvoicePaid struct {
	InvoiceID shared.InvoiceID
}

func (InvoicePaid) EventName() string { return NameInvoicePaid }
