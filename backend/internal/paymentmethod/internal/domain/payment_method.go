package domain

import (
	"errors"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

// MethodType は決済手段の種別。
type MethodType string

const (
	TypeCreditCard     MethodType = "credit_card"
	TypeBankAccount    MethodType = "bank_account"    // 口座振替
	TypePaymentSlip    MethodType = "payment_slip"    // 払込票（コンビニ）
	TypeVirtualAccount MethodType = "virtual_account" // 振込用バーチャル口座
)

// MethodStatus は決済手段の利用可否。
type MethodStatus string

const (
	MethodStatusActive    MethodStatus = "active"
	MethodStatusExpired   MethodStatus = "expired"
	MethodStatusSuspended MethodStatus = "suspended"
)

// RegistrationStatus は口座振替固有の登録状態機械。
// 「依頼を出した」≠「使える」という時間差を型で表現する。
type RegistrationStatus string

const (
	RegStatusPending   RegistrationStatus = "pending"   // 依頼受付
	RegStatusReviewing RegistrationStatus = "reviewing" // 銀行審査中
	RegStatusCompleted RegistrationStatus = "completed" // 登録完了（使用可能）
	RegStatusRejected  RegistrationStatus = "rejected"  // 否認
)

// PaymentMethod は決済手段の集約ルート。
// 生 PAN は持たず PSP トークンのみ保持する。
// BillingAccount に属し、Invoice（債権）とは無関係。
type PaymentMethod struct {
	ID               shared.PaymentMethodID
	BillingAccountID shared.BillingAccountID
	Type             MethodType
	Status           MethodStatus
	PSPToken         string
	Priority         int // BillingAccount 内での優先順位（小さいほど高優先）

	// TypeBankAccount のみ使用。nil = 非口座振替手段。
	registrationStatus *RegistrationStatus
}

var (
	ErrNotBankAccount       = errors.New("この操作は口座振替手段のみ有効です")
	ErrAlreadyCompleted     = errors.New("登録はすでに完了しています")
	ErrAlreadyRejected      = errors.New("登録は否認済みです")
	ErrRegistrationNotReady = errors.New("口座振替の登録が完了していないため使用できません")
	ErrNotReviewing         = errors.New("この操作は審査中状態からのみ実行できます")
)

func NewCreditCard(id shared.PaymentMethodID, accountID shared.BillingAccountID, pspToken string, priority int) *PaymentMethod {
	return &PaymentMethod{
		ID:               id,
		BillingAccountID: accountID,
		Type:             TypeCreditCard,
		Status:           MethodStatusActive,
		PSPToken:         pspToken,
		Priority:         priority,
	}
}

func NewBankAccount(id shared.PaymentMethodID, accountID shared.BillingAccountID, pspToken string, priority int) *PaymentMethod {
	s := RegStatusPending
	return &PaymentMethod{
		ID:                 id,
		BillingAccountID:   accountID,
		Type:               TypeBankAccount,
		Status:             MethodStatusSuspended, // 登録完了まで使用不可
		PSPToken:           pspToken,
		Priority:           priority,
		registrationStatus: &s,
	}
}

func NewPaymentSlip(id shared.PaymentMethodID, accountID shared.BillingAccountID, priority int) *PaymentMethod {
	return &PaymentMethod{
		ID:               id,
		BillingAccountID: accountID,
		Type:             TypePaymentSlip,
		Status:           MethodStatusActive,
		Priority:         priority,
	}
}

func NewVirtualAccount(id shared.PaymentMethodID, accountID shared.BillingAccountID, pspToken string, priority int) *PaymentMethod {
	return &PaymentMethod{
		ID:               id,
		BillingAccountID: accountID,
		Type:             TypeVirtualAccount,
		Status:           MethodStatusActive,
		PSPToken:         pspToken,
		Priority:         priority,
	}
}

// RegistrationStatus は口座振替の登録状態のコピーを返す（非口座振替の場合は nil）。
// ポインタを返すが内部フィールドとは別の値なので外部からの変更は反映されない。
func (p *PaymentMethod) RegistrationStatus() *RegistrationStatus {
	if p.registrationStatus == nil {
		return nil
	}
	s := *p.registrationStatus
	return &s
}

// StartReview は銀行審査開始（依頼受付→審査中）。
func (p *PaymentMethod) StartReview() error {
	if p.Type != TypeBankAccount {
		return ErrNotBankAccount
	}
	if *p.registrationStatus != RegStatusPending {
		return errors.New("審査開始は pending 状態からのみ可能です")
	}
	s := RegStatusReviewing
	p.registrationStatus = &s
	return nil
}

// CompleteRegistration は口座振替の銀行審査通過。reviewing からのみ遷移できる。
func (p *PaymentMethod) CompleteRegistration() error {
	if p.Type != TypeBankAccount {
		return ErrNotBankAccount
	}
	if *p.registrationStatus != RegStatusReviewing {
		return ErrNotReviewing
	}
	s := RegStatusCompleted
	p.registrationStatus = &s
	p.Status = MethodStatusActive
	return nil
}

// RevertCompletion は CompleteRegistration の補償遷移。発行失敗時のロールバックに使う。
func (p *PaymentMethod) RevertCompletion() {
	s := RegStatusReviewing
	p.registrationStatus = &s
	p.Status = MethodStatusSuspended
}

// RejectRegistration は口座振替の銀行審査否認。reviewing からのみ遷移できる。
func (p *PaymentMethod) RejectRegistration() error {
	if p.Type != TypeBankAccount {
		return ErrNotBankAccount
	}
	if *p.registrationStatus != RegStatusReviewing {
		return ErrNotReviewing
	}
	s := RegStatusRejected
	p.registrationStatus = &s
	return nil
}

// Expire はカード期限切れ等による失効。
func (p *PaymentMethod) Expire() {
	p.Status = MethodStatusExpired
}

// IsUsable は決済に使えるかどうか。
func (p *PaymentMethod) IsUsable() bool {
	if p.Status != MethodStatusActive {
		return false
	}
	if p.Type == TypeBankAccount && *p.registrationStatus != RegStatusCompleted {
		return false
	}
	return true
}
