// Package events はモジュール間の統合イベント（モジュール間の契約）を定義する。
// 各モジュールの「ドメイン内イベント」はモジュール private に閉じ、ここには出さない。
package events

import "github.com/kato0373i/subscope/backend/internal/shared"

const (
	NameBillingDue                       = "contract.BillingDue"
	NameInvoiceIssued                    = "billing.InvoiceIssued"
	NameChargeRequested                  = "collection.ChargeRequested"
	NameCollectionEscalated              = "collection.CollectionEscalated"
	NamePaymentSucceeded                 = "payment.PaymentSucceeded"
	NamePaymentPending                   = "payment.PaymentPending"
	NamePaymentFailed                    = "payment.PaymentFailed"
	NameInvoicePaid                      = "settlement.InvoicePaid"
	NamePaymentMethodRegistered          = "paymentmethod.PaymentMethodRegistered"
	NameBankAccountRegistrationCompleted = "paymentmethod.BankAccountRegistrationCompleted"
	NamePaymentMethodExpired             = "paymentmethod.PaymentMethodExpired"
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
// IdempotencyKey は課金試行を一意に識別し、再送時の二重決済を防ぐ。
type ChargeRequested struct {
	InvoiceID       shared.InvoiceID
	PaymentMethodID shared.PaymentMethodID
	Amount          shared.Money
	IdempotencyKey  shared.IdempotencyKey
}

func (ChargeRequested) EventName() string { return NameChargeRequested }

// PaymentSucceeded は決済試行の成功。payment が発行する。
type PaymentSucceeded struct {
	InvoiceID     shared.InvoiceID
	TransactionID shared.TransactionID
	Amount        shared.Money
}

func (PaymentSucceeded) EventName() string { return NamePaymentSucceeded }

// PaymentPending は後日確定待ちの決済試行。payment が発行する。
// クレカと違い口座振替・払込票は結果が後日確定するため、collection はこれを失敗扱いせず
// （手段を切り替えず）入金確定を待つ。確定の事実は後で settlement が取り込む（#11）。
type PaymentPending struct {
	InvoiceID       shared.InvoiceID
	TransactionID   shared.TransactionID
	PaymentMethodID shared.PaymentMethodID
	Amount          shared.Money
}

func (PaymentPending) EventName() string { return NamePaymentPending }

// PaymentFailed は決済試行の失敗。payment が発行し、collection が手段の切替を判断する。
type PaymentFailed struct {
	InvoiceID       shared.InvoiceID
	TransactionID   shared.TransactionID
	PaymentMethodID shared.PaymentMethodID
	Reason          string
}

func (PaymentFailed) EventName() string { return NamePaymentFailed }

// CollectionEscalated は全決済手段が尽きた回収案件のエスカレーション。collection が発行する。
// 受信側（督促・解約モジュール等）はこのイベントを機に次アクションを実行する。
type CollectionEscalated struct {
	CaseID    shared.CollectionCaseID
	InvoiceID shared.InvoiceID
	Amount    shared.Money
}

func (CollectionEscalated) EventName() string { return NameCollectionEscalated }

// InvoicePaid は入金の消込完了。settlement が発行する。
type InvoicePaid struct {
	InvoiceID shared.InvoiceID
}

func (InvoicePaid) EventName() string { return NameInvoicePaid }

// PaymentMethodRegistered は決済手段の登録完了。paymentmethod が発行する。
type PaymentMethodRegistered struct {
	PaymentMethodID  shared.PaymentMethodID
	BillingAccountID shared.BillingAccountID
	MethodType       string
}

func (PaymentMethodRegistered) EventName() string { return NamePaymentMethodRegistered }

// BankAccountRegistrationCompleted は口座振替の銀行審査通過。paymentmethod が発行する。
// 受信側（collection）はこのイベントで口座振替を回収手段として使えるようになる。
type BankAccountRegistrationCompleted struct {
	PaymentMethodID  shared.PaymentMethodID
	BillingAccountID shared.BillingAccountID
}

func (BankAccountRegistrationCompleted) EventName() string {
	return NameBankAccountRegistrationCompleted
}

// PaymentMethodExpired はカード期限切れ。paymentmethod が発行する。
// collection はこのイベントで当該手段を戦略から除外する。
type PaymentMethodExpired struct {
	PaymentMethodID  shared.PaymentMethodID
	BillingAccountID shared.BillingAccountID
}

func (PaymentMethodExpired) EventName() string { return NamePaymentMethodExpired }
