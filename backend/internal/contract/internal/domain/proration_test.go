package domain

import (
	"testing"
	"time"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

// 月次契約のちょうど中間でプラン変更すると、旧プランの半額をクレジットし新プランの半額を課金する。
func TestProration_MidPeriodUpgrade(t *testing.T) {
	loc := time.UTC
	c := &Contract{
		MonthlyFee:      shared.JPY(3000),
		BillingCycle:    CycleMonthly,
		BillingAnchor:   BillingAnchor(1),
		NextBillingDate: time.Date(2026, 7, 1, 0, 0, 0, 0, loc), // 期間: 6/1〜7/1（30日）
		Status:          StatusActive,
	}
	// 6/16 に変更 → 残り 15 日（30 日中）。
	changeDate := time.Date(2026, 6, 16, 0, 0, 0, 0, loc)

	adj, err := ProrationPolicy{}.Calculate(c, shared.JPY(5000), changeDate)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if adj.DaysInPeriod != 30 || adj.DaysRemaining != 15 {
		t.Fatalf("日数 = %d/%d, want 15/30", adj.DaysRemaining, adj.DaysInPeriod)
	}
	if adj.Credit.Amount != 1500 {
		t.Errorf("Credit = %d, want 1500（旧プラン 3000 の半分）", adj.Credit.Amount)
	}
	if adj.Charge.Amount != 2500 {
		t.Errorf("Charge = %d, want 2500（新プラン 5000 の半分）", adj.Charge.Amount)
	}
	if adj.Net.Amount != 1000 {
		t.Errorf("Net = %d, want 1000（差額の追加請求）", adj.Net.Amount)
	}
}

// ダウングレードでは Net が負（返金）になる。
func TestProration_Downgrade_NetNegative(t *testing.T) {
	loc := time.UTC
	c := &Contract{
		MonthlyFee:      shared.JPY(5000),
		BillingCycle:    CycleMonthly,
		BillingAnchor:   BillingAnchor(1),
		NextBillingDate: time.Date(2026, 7, 1, 0, 0, 0, 0, loc),
		Status:          StatusActive,
	}
	changeDate := time.Date(2026, 6, 16, 0, 0, 0, 0, loc)
	adj, err := ProrationPolicy{}.Calculate(c, shared.JPY(3000), changeDate)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if !adj.Net.IsNegative() {
		t.Errorf("Net = %d, want 負（返金）", adj.Net.Amount)
	}
}

// 期間末日での変更は残日数 0 → 調整なし。
func TestProration_AtPeriodEnd_NoAdjustment(t *testing.T) {
	loc := time.UTC
	c := &Contract{
		MonthlyFee:      shared.JPY(3000),
		BillingCycle:    CycleMonthly,
		BillingAnchor:   BillingAnchor(1),
		NextBillingDate: time.Date(2026, 7, 1, 0, 0, 0, 0, loc),
		Status:          StatusActive,
	}
	adj, err := ProrationPolicy{}.Calculate(c, shared.JPY(5000), time.Date(2026, 7, 1, 0, 0, 0, 0, loc))
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if adj.DaysRemaining != 0 || adj.Net.Amount != 0 {
		t.Errorf("期間末: remaining=%d net=%d, want 0/0", adj.DaysRemaining, adj.Net.Amount)
	}
}

// 現在の請求期間より前の changeDate は弾く（前期間に属する変更の誤計算を防ぐ）。
func TestProration_RejectsChangeDateBeforePeriod(t *testing.T) {
	loc := time.UTC
	c := &Contract{
		MonthlyFee:      shared.JPY(3000),
		BillingCycle:    CycleMonthly,
		BillingAnchor:   BillingAnchor(1),
		NextBillingDate: time.Date(2026, 7, 1, 0, 0, 0, 0, loc), // 期間: 6/1〜7/1
		Status:          StatusActive,
	}
	// 5/20 は前期間 → 不正。
	if _, err := (ProrationPolicy{}).Calculate(c, shared.JPY(5000), time.Date(2026, 5, 20, 0, 0, 0, 0, loc)); err != ErrInvalidPeriod {
		t.Errorf("期間前の変更は ErrInvalidPeriod: got %v", err)
	}
	// 7/2 は次期間 → 不正。
	if _, err := (ProrationPolicy{}).Calculate(c, shared.JPY(5000), time.Date(2026, 7, 2, 0, 0, 0, 0, loc)); err != ErrInvalidPeriod {
		t.Errorf("期間後の変更は ErrInvalidPeriod: got %v", err)
	}
}

// 通貨不一致は弾く。
func TestProration_RejectsCurrencyMismatch(t *testing.T) {
	loc := time.UTC
	c := &Contract{
		MonthlyFee:      shared.JPY(3000),
		BillingCycle:    CycleMonthly,
		BillingAnchor:   BillingAnchor(1),
		NextBillingDate: time.Date(2026, 7, 1, 0, 0, 0, 0, loc),
		Status:          StatusActive,
	}
	usd := shared.Money{Amount: 5000, Currency: "USD"}
	_, err := ProrationPolicy{}.Calculate(c, usd, time.Date(2026, 6, 16, 0, 0, 0, 0, loc))
	if err != ErrCurrencyMismatch {
		t.Errorf("通貨不一致は ErrCurrencyMismatch: got %v", err)
	}
}

// ChangePlan はキャンセル済み契約では不可。
func TestChangePlan_RejectsCancelled(t *testing.T) {
	c := New("CT-1", "MEM-1", "BA-1", shared.JPY(3000))
	_ = c.Cancel()
	if err := c.ChangePlan("PLAN-2", shared.JPY(5000)); err != ErrAlreadyCancelled {
		t.Errorf("キャンセル済みの ChangePlan は ErrAlreadyCancelled: got %v", err)
	}
}
