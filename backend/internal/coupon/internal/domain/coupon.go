// Package domain は coupon モジュールの集約・状態を閉じ込める private ドメイン層。
// 依存は shared と標準ライブラリのみ。
package domain

import (
	"errors"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

// DiscountType は割引の種類。
type DiscountType string

const (
	DiscountAmount  DiscountType = "amount"  // 定額割引（円）
	DiscountPercent DiscountType = "percent" // 定率割引（%）
)

var (
	ErrEmptyCode        = errors.New("クーポンコードは必須です")
	ErrInvalidValue     = errors.New("割引値が不正です")
	ErrInvalidType      = errors.New("割引種別が不正です")
	ErrAlreadyRedeemed = errors.New("この請求先は既にクーポンを利用済みです")
	ErrRedemptionLimit = errors.New("クーポンの利用上限に達しました")
)

// Coupon はクーポンの集約。利用（Redemption）を記録し、二重利用と利用上限を防ぐ。
type Coupon struct {
	ID             shared.CouponID
	OrgID          shared.OrgID
	Code           string
	discountType   DiscountType
	value          int64
	maxRedemptions int // 0 は無制限
	redeemed       map[shared.BillingAccountID]bool
}

// New はクーポンを生成する。定率は 1〜100%、定額は正の金額のみ許可する。
func New(id shared.CouponID, orgID shared.OrgID, code string, dt DiscountType, value int64, maxRedemptions int) (*Coupon, error) {
	if code == "" {
		return nil, ErrEmptyCode
	}
	switch dt {
	case DiscountAmount:
		if value <= 0 {
			return nil, ErrInvalidValue
		}
	case DiscountPercent:
		if value <= 0 || value > 100 {
			return nil, ErrInvalidValue
		}
	default:
		return nil, ErrInvalidType
	}
	if maxRedemptions < 0 {
		return nil, ErrInvalidValue
	}
	return &Coupon{
		ID:             id,
		OrgID:          orgID,
		Code:           code,
		discountType:   dt,
		value:          value,
		maxRedemptions: maxRedemptions,
		redeemed:       make(map[shared.BillingAccountID]bool),
	}, nil
}

// Redeem は請求先によるクーポン利用を記録する。
// 同一請求先の二重利用、および利用上限超過を不変条件として弾く。
func (c *Coupon) Redeem(account shared.BillingAccountID) error {
	if c.redeemed[account] {
		return ErrAlreadyRedeemed
	}
	if c.maxRedemptions > 0 && len(c.redeemed) >= c.maxRedemptions {
		return ErrRedemptionLimit
	}
	c.redeemed[account] = true
	return nil
}

// RedeemedCount は利用済み件数。
func (c *Coupon) RedeemedCount() int { return len(c.redeemed) }

// Apply は金額に割引を適用した後の金額を返す（0 円未満にはならない）。
// 端数は切り捨て。通貨はそのまま保持する。
func (c *Coupon) Apply(amount shared.Money) (shared.Money, error) {
	var discount int64
	switch c.discountType {
	case DiscountAmount:
		discount = c.value
	case DiscountPercent:
		discount = amount.Amount * c.value / 100
	default:
		return shared.Money{}, ErrInvalidType
	}
	result := amount.Amount - discount
	if result < 0 {
		result = 0
	}
	return shared.Money{Amount: result, Currency: amount.Currency}, nil
}
