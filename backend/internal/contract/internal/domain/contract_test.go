package domain

import (
	"testing"
	"time"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

func TestNew_StartsActiveWithBothReferences(t *testing.T) {
	c := New("CT-1", "MEM-1", "BA-1", shared.JPY(3000))

	if c.Status != StatusActive {
		t.Errorf("Status = %q, want %q", c.Status, StatusActive)
	}
	// 会員 ≠ 支払者：契約は両方を参照する。
	if c.MemberID != "MEM-1" {
		t.Errorf("MemberID = %q, want MEM-1", c.MemberID)
	}
	if c.BillingAccountID != "BA-1" {
		t.Errorf("BillingAccountID = %q, want BA-1", c.BillingAccountID)
	}
	if c.MonthlyFee.Amount != 3000 {
		t.Errorf("MonthlyFee = %v, want 3000", c.MonthlyFee)
	}
}

func TestContract_StatusTransitions(t *testing.T) {
	c := New("CT-001", "MEM-001", "BA-001", shared.JPY(1000))
	if c.Status != StatusActive {
		t.Fatalf("初期状態は active でなければならない: got %s", c.Status)
	}

	// active → past_due
	if err := c.SetPastDue(); err != nil {
		t.Fatalf("SetPastDue: %v", err)
	}
	if c.Status != StatusPastDue {
		t.Fatalf("SetPastDue 後は past_due でなければならない: got %s", c.Status)
	}

	// past_due → suspended
	if err := c.Suspend(); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if c.Status != StatusSuspended {
		t.Fatalf("Suspend 後は suspended でなければならない: got %s", c.Status)
	}

	// cancelled へ
	if err := c.Cancel(); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if c.Status != StatusCancelled {
		t.Fatalf("Cancel 後は cancelled でなければならない: got %s", c.Status)
	}

	// cancelled から再 Cancel はエラー
	if err := c.Cancel(); err != ErrAlreadyCancelled {
		t.Fatalf("二重 Cancel は ErrAlreadyCancelled を返すべき: got %v", err)
	}
}

func TestContract_InvalidTransition(t *testing.T) {
	c := New("CT-002", "MEM-001", "BA-001", shared.JPY(1000))
	// active から直接 Suspend はエラー
	if err := c.Suspend(); err != ErrInvalidTransition {
		t.Fatalf("active → Suspend は ErrInvalidTransition を返すべき: got %v", err)
	}
}

func TestContract_TrialingActivate(t *testing.T) {
	c := NewFull("CT-003", "ORG-001", "MEM-001", "BA-001", "PLAN-001",
		shared.JPY(1000), CycleMonthly, 1, TrialPeriod{Days: 14})
	if c.Status != StatusTrialing {
		t.Fatalf("トライアルあり契約の初期状態は trialing でなければならない: got %s", c.Status)
	}
	if err := c.Activate(); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if c.Status != StatusActive {
		t.Fatalf("Activate 後は active でなければならない: got %s", c.Status)
	}
}

func TestContract_NextBillingDate_Monthly(t *testing.T) {
	// 月次請求、起算日=1 の場合、翌月1日が次回請求日
	loc := time.UTC
	from := time.Date(2026, 6, 15, 0, 0, 0, 0, loc)
	c := &Contract{BillingCycle: CycleMonthly, BillingAnchor: 1}
	next := c.calcNextBillingDate(from)
	want := time.Date(2026, 7, 1, 0, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("次回請求日: got %v, want %v", next, want)
	}
}

func TestContract_NextBillingDate_MonthEnd(t *testing.T) {
	// 月次請求、起算日=31 の場合、翌月末日が次回請求日
	loc := time.UTC
	from := time.Date(2026, 1, 31, 0, 0, 0, 0, loc)
	c := &Contract{BillingCycle: CycleMonthly, BillingAnchor: 31}
	next := c.calcNextBillingDate(from)
	// 2月は28日（2026年は平年）
	want := time.Date(2026, 2, 28, 0, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("月末起算の次回請求日: got %v, want %v", next, want)
	}
}

func TestContract_NextBillingDate_Quarterly(t *testing.T) {
	// 四半期請求、起算日=15 の場合、3ヶ月後の15日が次回請求日
	loc := time.UTC
	from := time.Date(2026, 1, 15, 0, 0, 0, 0, loc)
	c := &Contract{BillingCycle: CycleQuarterly, BillingAnchor: 15}
	next := c.calcNextBillingDate(from)
	want := time.Date(2026, 4, 15, 0, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("四半期次回請求日: got %v, want %v", next, want)
	}
}

func TestContract_NextBillingDate_Annual(t *testing.T) {
	// 年次請求、起算日=1 の場合、1年後の同月1日が次回請求日
	loc := time.UTC
	from := time.Date(2026, 3, 1, 0, 0, 0, 0, loc)
	c := &Contract{BillingCycle: CycleAnnual, BillingAnchor: 1}
	next := c.calcNextBillingDate(from)
	want := time.Date(2027, 3, 1, 0, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("年次次回請求日: got %v, want %v", next, want)
	}
}

func TestContract_NewFull_AnchorNormalization(t *testing.T) {
	// anchor=0 → 1 に正規化
	c0 := NewFull("CT-N01", "ORG-001", "MEM-001", "BA-001", "PLAN-001",
		shared.JPY(1000), CycleMonthly, 0, TrialPeriod{})
	if c0.BillingAnchor != 1 {
		t.Fatalf("anchor 0 は 1 に正規化されるべき: got %d", c0.BillingAnchor)
	}

	// anchor=32 → 31 に正規化
	c32 := NewFull("CT-N02", "ORG-001", "MEM-001", "BA-001", "PLAN-001",
		shared.JPY(1000), CycleMonthly, 32, TrialPeriod{})
	if c32.BillingAnchor != 31 {
		t.Fatalf("anchor 32 は 31 に正規化されるべき: got %d", c32.BillingAnchor)
	}
}

func TestContract_NewFull_InvalidCycleFallback(t *testing.T) {
	c := NewFull("CT-N03", "ORG-001", "MEM-001", "BA-001", "PLAN-001",
		shared.JPY(1000), BillingCycle("unknown"), 1, TrialPeriod{})
	if c.BillingCycle != CycleMonthly {
		t.Fatalf("未知の BillingCycle は monthly にフォールバックされるべき: got %s", c.BillingCycle)
	}
}
