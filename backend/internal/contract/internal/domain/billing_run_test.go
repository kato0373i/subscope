package domain

import (
	"testing"
	"time"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

// 指定 anchor の契約を作り、状態と次回請求日を任意に設定するヘルパ。
func dueContract(id shared.ContractID, status Status, next time.Time) *Contract {
	return &Contract{
		ID:               id,
		BillingAccountID: shared.BillingAccountID("BA-" + string(id)),
		MonthlyFee:       shared.JPY(3000),
		Status:           status,
		BillingCycle:     CycleMonthly,
		BillingAnchor:    BillingAnchor(next.Day()),
		NextBillingDate:  next,
	}
}

// Plan は「課金対象 かつ 次回請求日 <= asOf」の契約だけを抽出する。
func TestBillingRun_PlanSelectsDueBillable(t *testing.T) {
	asOf := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	past := asOf.AddDate(0, 0, -1)
	future := asOf.AddDate(0, 0, 1)

	contracts := []*Contract{
		dueContract("active-due", StatusActive, past),      // ○ 対象
		dueContract("pastdue-due", StatusPastDue, asOf),    // ○ 対象（同日含む）
		dueContract("active-future", StatusActive, future), // × まだ来ていない
		dueContract("trialing", StatusTrialing, past),      // × 課金対象外
		dueContract("suspended", StatusSuspended, past),    // × 課金対象外
		dueContract("cancelled", StatusCancelled, past),    // × 課金対象外
	}

	items := NewBillingRun(asOf).Plan(contracts)

	if len(items) != 2 {
		t.Fatalf("抽出件数 = %d, want 2 (%+v)", len(items), items)
	}
	got := map[shared.ContractID]BillingItem{}
	for _, it := range items {
		got[it.ContractID] = it
	}
	if _, ok := got["active-due"]; !ok {
		t.Error("active-due が抽出されていない")
	}
	if _, ok := got["pastdue-due"]; !ok {
		t.Error("pastdue-due が抽出されていない")
	}
	// 抽出結果は「いくら・どの期間」を持つ（決済手段はそもそも型に存在しない）。
	if got["active-due"].Amount != shared.JPY(3000) {
		t.Errorf("Amount = %v, want 3000", got["active-due"].Amount)
	}
	if got["pastdue-due"].Period != "2026-06" {
		t.Errorf("Period = %q, want 2026-06", got["pastdue-due"].Period)
	}
}

// Plan は副作用を持たない（次回請求日を進めない）ため、ドライランに使える。
func TestBillingRun_PlanIsSideEffectFree(t *testing.T) {
	asOf := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	next := asOf.AddDate(0, 0, -1)
	c := dueContract("CT-1", StatusActive, next)

	_ = NewBillingRun(asOf).Plan([]*Contract{c})

	if !c.NextBillingDate.Equal(next) {
		t.Errorf("Plan が NextBillingDate を変更した: %v, want %v", c.NextBillingDate, next)
	}
}

// 同一 asOf の Run は安定した ID を返す（冪等な実行識別・監査用）。
func TestBillingRun_StableID(t *testing.T) {
	asOf := time.Date(2026, 6, 10, 9, 30, 0, 0, time.UTC)
	if id := NewBillingRun(asOf).ID(); id != "BR-20260610" {
		t.Errorf("ID = %q, want BR-20260610", id)
	}
}
