package coupon_test

import (
	"testing"

	"github.com/kato0373i/subscope/backend/internal/coupon"
	"github.com/kato0373i/subscope/backend/internal/shared"
)

func TestService_RegisterRedeemApply(t *testing.T) {
	s := coupon.NewService()
	if err := s.Register("CPN-1", "ORG-1", "WELCOME", coupon.DiscountPercent, 10, 100); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := s.Redeem("CPN-1", "BA-1"); err != nil {
		t.Fatalf("Redeem: %v", err)
	}
	got, err := s.Apply("CPN-1", shared.JPY(3000))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got.Amount != 2700 {
		t.Errorf("Apply = %d, want 2700", got.Amount)
	}
}

func TestService_RejectsDuplicateAndDoubleRedeem(t *testing.T) {
	s := coupon.NewService()
	_ = s.Register("CPN-1", "ORG-1", "WELCOME", coupon.DiscountAmount, 500, 0)

	if err := s.Register("CPN-1", "ORG-1", "X", coupon.DiscountAmount, 100, 0); err != coupon.ErrDuplicateCoupon {
		t.Errorf("重複登録は ErrDuplicateCoupon: got %v", err)
	}
	if err := s.Redeem("CPN-1", "BA-1"); err != nil {
		t.Fatalf("Redeem#1: %v", err)
	}
	if err := s.Redeem("CPN-1", "BA-1"); err == nil {
		t.Error("二重利用はエラーになるべき")
	}
}

func TestService_NotFound(t *testing.T) {
	s := coupon.NewService()
	if err := s.Redeem("CPN-X", "BA-1"); err != coupon.ErrNotFound {
		t.Errorf("未登録 Redeem は ErrNotFound: got %v", err)
	}
	if _, err := s.Apply("CPN-X", shared.JPY(1000)); err != coupon.ErrNotFound {
		t.Errorf("未登録 Apply は ErrNotFound: got %v", err)
	}
}
