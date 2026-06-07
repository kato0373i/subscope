package domain

import (
	"testing"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

func TestNew_Validations(t *testing.T) {
	if _, err := New("CPN-1", "ORG-1", "", DiscountAmount, 500, 0); err != ErrEmptyCode {
		t.Errorf("空コードは ErrEmptyCode: got %v", err)
	}
	if _, err := New("CPN-1", "ORG-1", "X", DiscountPercent, 150, 0); err != ErrInvalidValue {
		t.Errorf("100%%超は ErrInvalidValue: got %v", err)
	}
	if _, err := New("CPN-1", "ORG-1", "X", "other", 10, 0); err != ErrInvalidType {
		t.Errorf("不正種別は ErrInvalidType: got %v", err)
	}
}

// 同一請求先の二重利用を弾く。
func TestRedeem_PreventsDoubleUse(t *testing.T) {
	c, err := New("CPN-1", "ORG-1", "WELCOME", DiscountPercent, 10, 0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.Redeem("BA-1"); err != nil {
		t.Fatalf("Redeem#1: %v", err)
	}
	if err := c.Redeem("BA-1"); err != ErrAlreadyRedeemed {
		t.Errorf("二重利用は ErrAlreadyRedeemed: got %v", err)
	}
	if c.RedeemedCount() != 1 {
		t.Errorf("RedeemedCount = %d, want 1", c.RedeemedCount())
	}
}

// mustNew はクーポン生成に失敗したらテストを止めるヘルパー。
func mustNew(t *testing.T, id shared.CouponID, code string, dt DiscountType, value int64, max int) *Coupon {
	t.Helper()
	c, err := New(id, "ORG-1", code, dt, value, max)
	if err != nil {
		t.Fatalf("New(%s): %v", code, err)
	}
	return c
}

// 利用上限を超える利用を弾く。
func TestRedeem_EnforcesLimit(t *testing.T) {
	c := mustNew(t, "CPN-1", "LTD", DiscountAmount, 500, 2)
	if err := c.Redeem("BA-1"); err != nil {
		t.Fatalf("Redeem BA-1: %v", err)
	}
	if err := c.Redeem("BA-2"); err != nil {
		t.Fatalf("Redeem BA-2: %v", err)
	}
	if err := c.Redeem("BA-3"); err != ErrRedemptionLimit {
		t.Errorf("上限超過は ErrRedemptionLimit: got %v", err)
	}
}

func TestApply_PercentAndAmountAndFloor(t *testing.T) {
	pct := mustNew(t, "CPN-1", "P10", DiscountPercent, 10, 0)
	got, err := pct.Apply(shared.JPY(3000))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got.Amount != 2700 {
		t.Errorf("10%%引き = %d, want 2700", got.Amount)
	}

	amt := mustNew(t, "CPN-2", "A500", DiscountAmount, 500, 0)
	if got, _ := amt.Apply(shared.JPY(3000)); got.Amount != 2500 {
		t.Errorf("500 円引き = %d, want 2500", got.Amount)
	}

	// 割引が額面を超えても 0 円止まり。
	big := mustNew(t, "CPN-3", "A9999", DiscountAmount, 9999, 0)
	if got, _ := big.Apply(shared.JPY(3000)); got.Amount != 0 {
		t.Errorf("過大割引 = %d, want 0", got.Amount)
	}
}

// 大きな金額でも定率割引で int64 オーバーフローせず正しく計算する。
func TestApply_PercentNoOverflow(t *testing.T) {
	c := mustNew(t, "CPN-1", "P50", DiscountPercent, 50, 0)
	huge := shared.Money{Amount: 9_000_000_000_000_000_000, Currency: "JPY"} // MaxInt64/100 超
	got, err := c.Apply(huge)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := huge.Amount - huge.Amount/2 // 50% 引き
	if got.Amount != want {
		t.Errorf("50%%引き = %d, want %d", got.Amount, want)
	}
}
