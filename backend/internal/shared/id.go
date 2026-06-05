package shared

// モジュール間の参照はすべてこの型付き ID 経由で行う。
// 他モジュールの集約（構造体）を直接 import することは禁止。
type (
	OrgID            string
	MemberID         string
	BillingAccountID string
	ContractID       string
	InvoiceID        string
	PaymentMethodID  string
	TransactionID    string
	CollectionCaseID string
	SettlementID     string
)
