package shared

// モジュール間の参照はすべてこの型付き ID 経由で行う。
// 他モジュールの集約（構造体）を直接 import することは禁止。
type (
	OrgID             string
	MemberID          string
	BillingAccountID  string
	ContractID        string
	PlanID            string
	InvoiceID         string
	PaymentMethodID   string
	TransactionID     string
	CollectionCaseID  string
	SettlementID      string
	DunningCampaignID string
	NotificationID    string
	CouponID          string
	CreditNoteID      string
)

// IdempotencyKey は冪等性キー。PSP の Webhook 二重通知やイベント再送に備え、
// 同一の論理操作（課金・入金消込）を一度だけ処理するために使う。
type IdempotencyKey string
