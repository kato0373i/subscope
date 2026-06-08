// Package events はモジュール間の統合イベント（モジュール間の契約）を定義する。
// 各モジュールの「ドメイン内イベント」はモジュール private に閉じ、ここには出さない。
package events

import "github.com/kato0373i/subscope/backend/internal/shared"

const (
	NameBillingDue                       = "contract.BillingDue"
	NameContractActivated                = "contract.ContractActivated"
	NamePlanChanged                      = "contract.PlanChanged"
	NameContractSuspended                = "contract.ContractSuspended"
	NameContractCancelled                = "contract.ContractCancelled"
	NameInvoiceIssued                    = "billing.InvoiceIssued"
	NameChargeRequested                  = "collection.ChargeRequested"
	NameCollectionEscalated              = "collection.CollectionEscalated"
	NameCollectionRecovered              = "collection.CollectionRecovered"
	NameCollectionWrittenOff             = "collection.CollectionWrittenOff"
	NamePaymentSucceeded                 = "payment.PaymentSucceeded"
	NamePaymentPending                   = "payment.PaymentPending"
	NamePaymentFailed                    = "payment.PaymentFailed"
	NameInvoicePaid                      = "settlement.InvoicePaid"
	NameCreditNoteIssued                 = "creditnote.CreditNoteIssued"
	NameInvoicePartiallyPaid             = "settlement.InvoicePartiallyPaid"
	NameUnmatchedDepositDetected         = "settlement.UnmatchedDepositDetected"
	NameDunningStepTriggered             = "dunning.DunningStepTriggered"
	NameNotificationSent                 = "notification.NotificationSent"
	NamePaymentMethodRegistered          = "paymentmethod.PaymentMethodRegistered"
	NameBankAccountRegistrationCompleted = "paymentmethod.BankAccountRegistrationCompleted"
	NamePaymentMethodExpired             = "paymentmethod.PaymentMethodExpired"
)

// AllNames はすべての統合イベント名を返す。audit / webhook のように
// 全イベントを横断して購読するモジュールが利用する。
// 新しい統合イベントを追加したらここにも追記する。
func AllNames() []string {
	return []string{
		NameBillingDue,
		NameContractActivated,
		NamePlanChanged,
		NameContractSuspended,
		NameContractCancelled,
		NameInvoiceIssued,
		NameChargeRequested,
		NameCollectionEscalated,
		NameCollectionRecovered,
		NameCollectionWrittenOff,
		NamePaymentSucceeded,
		NamePaymentPending,
		NamePaymentFailed,
		NameInvoicePaid,
		NameCreditNoteIssued,
		NameInvoicePartiallyPaid,
		NameUnmatchedDepositDetected,
		NameDunningStepTriggered,
		NameNotificationSent,
		NamePaymentMethodRegistered,
		NameBankAccountRegistrationCompleted,
		NamePaymentMethodExpired,
	}
}

// BillingDue は契約の請求サイクル到来。contract が発行する。
type BillingDue struct {
	ContractID       shared.ContractID
	BillingAccountID shared.BillingAccountID
	Amount           shared.Money
	Period           string
}

func (BillingDue) EventName() string { return NameBillingDue }

// ContractActivated はトライアル終了等で契約が有効化されたこと。contract が発行する。
type ContractActivated struct {
	ContractID shared.ContractID
}

func (ContractActivated) EventName() string { return NameContractActivated }

// PlanChanged はプラン変更。contract が発行する。
// NetAdjustment は日割り差額（正なら追加請求、負なら返金）。billing が調整明細を起票する。
type PlanChanged struct {
	ContractID    shared.ContractID
	OldPlanID     shared.PlanID
	NewPlanID     shared.PlanID
	NetAdjustment shared.Money
}

func (PlanChanged) EventName() string { return NamePlanChanged }

// ContractSuspended は契約の利用停止。contract が発行する。
type ContractSuspended struct {
	ContractID shared.ContractID
}

func (ContractSuspended) EventName() string { return NameContractSuspended }

// ContractCancelled は契約の解約。contract が発行する。
type ContractCancelled struct {
	ContractID shared.ContractID
}

func (ContractCancelled) EventName() string { return NameContractCancelled }

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
// PlannedActions は戦略が定めたエスカレーション手順（例: notify→suspend→request_cancel）。
// 受信側はこの順に督促・利用停止・解約申請を進める。collection 自身は実行しない。
type CollectionEscalated struct {
	CaseID         shared.CollectionCaseID
	InvoiceID      shared.InvoiceID
	Amount         shared.Money
	PlannedActions []string
}

func (CollectionEscalated) EventName() string { return NameCollectionEscalated }

// CollectionRecovered は未収案件が入金消込により回収完了したこと。collection が発行する。
// metrics（回収率の集計）や督促の取り下げトリガーとして使う。
type CollectionRecovered struct {
	CaseID    shared.CollectionCaseID
	InvoiceID shared.InvoiceID
	Amount    shared.Money
}

func (CollectionRecovered) EventName() string { return NameCollectionRecovered }

// CollectionWrittenOff は回収を諦め債権を貸倒として落としたこと。collection が発行する。
// 少額債権を延々追わないための「諦めライン」を超えた案件で発火する。
type CollectionWrittenOff struct {
	CaseID    shared.CollectionCaseID
	InvoiceID shared.InvoiceID
	Amount    shared.Money
	Reason    string
}

func (CollectionWrittenOff) EventName() string { return NameCollectionWrittenOff }

// InvoicePaid は入金の消込完了（全額充当）。settlement が発行する。
type InvoicePaid struct {
	InvoiceID shared.InvoiceID
}

func (InvoicePaid) EventName() string { return NameInvoicePaid }

// InvoicePartiallyPaid は請求への部分消込。settlement が発行する。
// 入金が請求額に満たない（または団体一括の一部充当）場合に発火し、RemainingAmount に残額を載せる。
type InvoicePartiallyPaid struct {
	InvoiceID       shared.InvoiceID
	PaidAmount      shared.Money
	RemainingAmount shared.Money
}

func (InvoicePartiallyPaid) EventName() string { return NameInvoicePartiallyPaid }

// UnmatchedDepositDetected は自動消込できなかった入金。settlement が発行する。
// 名義の揺れ・金額不一致・該当請求なしなどが原因で、手動消込（オペレータ対応）を要する。
type UnmatchedDepositDetected struct {
	Reference string
	Account   shared.BillingAccountID
	PayerName string
	Amount    shared.Money
}

func (UnmatchedDepositDetected) EventName() string { return NameUnmatchedDepositDetected }

// DunningStepTriggered は督促シーケンスの 1 ステップ発火。dunning が発行する。
// notification が購読し、指定チャネル（email/SMS/郵送）で実際の送信を行う。
type DunningStepTriggered struct {
	CampaignID shared.DunningCampaignID
	InvoiceID  shared.InvoiceID
	Account    shared.BillingAccountID
	Channel    string
	StepNumber int
}

func (DunningStepTriggered) EventName() string { return NameDunningStepTriggered }

// NotificationSent は通知の送信完了。notification が発行する。
type NotificationSent struct {
	NotificationID shared.NotificationID
	InvoiceID      shared.InvoiceID
	Account        shared.BillingAccountID
	Channel        string
}

func (NotificationSent) EventName() string { return NameNotificationSent }

// CreditNoteIssued は赤伝（適格返還請求書＝CreditNote）の発行。creditnote が発行する。
// 元 Invoice への返金・取消を表す独立文書。Amount は返金額（正の値）。
type CreditNoteIssued struct {
	CreditNoteID shared.CreditNoteID
	ContractID   shared.ContractID
	Amount       shared.Money
	Reason       string
}

func (CreditNoteIssued) EventName() string { return NameCreditNoteIssued }

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
