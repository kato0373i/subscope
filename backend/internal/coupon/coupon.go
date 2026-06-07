// Package coupon はクーポンマスタモジュールの公開 API。
// クーポン（Coupon / Redemption）を管理し、二重利用と利用上限を防ぐ。
package coupon

import (
	"errors"
	"sync"

	"github.com/kato0373i/subscope/backend/internal/coupon/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
)

// 内部ドメイン型を再エクスポートする。
type DiscountType = domain.DiscountType

const (
	DiscountAmount  = domain.DiscountAmount
	DiscountPercent = domain.DiscountPercent
)

var (
	// ErrDuplicateCoupon はクーポン ID が重複した場合に返る。
	ErrDuplicateCoupon = errors.New("クーポン ID が重複しています")
	// ErrNotFound はクーポンが見つからない場合に返る。
	ErrNotFound = errors.New("クーポンが見つかりません")
)

type Service struct {
	mu      sync.Mutex
	coupons map[shared.CouponID]*domain.Coupon
}

func NewService() *Service {
	return &Service{coupons: make(map[shared.CouponID]*domain.Coupon)}
}

// Register は新しいクーポンを登録する。
func (s *Service) Register(id shared.CouponID, orgID shared.OrgID, code string, dt DiscountType, value int64, maxRedemptions int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.coupons[id]; ok {
		return ErrDuplicateCoupon
	}
	c, err := domain.New(id, orgID, code, dt, value, maxRedemptions)
	if err != nil {
		return err
	}
	s.coupons[id] = c
	return nil
}

// Redeem は請求先によるクーポン利用を記録する。二重利用・上限超過はエラー。
func (s *Service) Redeem(id shared.CouponID, account shared.BillingAccountID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.coupons[id]
	if !ok {
		return ErrNotFound
	}
	return c.Redeem(account)
}

// Apply はクーポンの割引を金額に適用した後の金額を返す。
func (s *Service) Apply(id shared.CouponID, amount shared.Money) (shared.Money, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.coupons[id]
	if !ok {
		return shared.Money{}, ErrNotFound
	}
	return c.Apply(amount)
}
