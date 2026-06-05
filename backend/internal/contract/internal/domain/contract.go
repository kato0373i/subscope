// Package domain は contract モジュールの内部ドメイン。
// パスに internal を含むため、contract モジュールの外からは import できない（Go の機構で強制）。
package domain

import "github.com/kato0373i/subscope/backend/internal/shared"

type Status string

const (
	StatusTrialing  Status = "trialing"
	StatusActive    Status = "active"
	StatusPastDue   Status = "past_due"
	StatusSuspended Status = "suspended"
	StatusCancelled Status = "cancelled"
)

// Contract は会員(Member)と支払者(BillingAccount)の両方を参照する契約。
type Contract struct {
	ID               shared.ContractID
	MemberID         shared.MemberID
	BillingAccountID shared.BillingAccountID
	Status           Status
	MonthlyFee       shared.Money
}

func New(id shared.ContractID, member shared.MemberID, account shared.BillingAccountID, fee shared.Money) *Contract {
	return &Contract{
		ID:               id,
		MemberID:         member,
		BillingAccountID: account,
		Status:           StatusActive,
		MonthlyFee:       fee,
	}
}
