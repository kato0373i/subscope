package domain

import (
	"testing"
	"time"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

func TestNewCase_StartsInProgress(t *testing.T) {
	c := NewCase("CASE-1", "INV-1", shared.JPY(3000), DefaultStrategy())
	if c.Status != StatusInProgress {
		t.Errorf("Status = %q, want %q", c.Status, StatusInProgress)
	}
}

// 起票直後の Start は最優先手段で Retry を返す（待機なし）。
func TestStart_UsesFirstMethodWithoutBackoff(t *testing.T) {
	c := NewCase("CASE-1", "INV-1", shared.JPY(3000), DefaultStrategy())
	d := c.Start()
	if d.Kind != DecisionRetry {
		t.Fatalf("Kind = %v, want Retry", d.Kind)
	}
	if d.Method != "PM-card-primary" {
		t.Errorf("Method = %q, want PM-card-primary", d.Method)
	}
	if d.Backoff != 0 {
		t.Errorf("初回 Backoff = %s, want 0", d.Backoff)
	}
}

// 失敗のたびに戦略の優先順どおり次の手段へ進み、間隔は指数的に伸びる。
func TestRecordFailure_FollowsFallbackOrderWithBackoff(t *testing.T) {
	c := NewCase("CASE-1", "INV-1", shared.JPY(3000), DefaultStrategy())
	c.Start()

	wantMethods := []shared.PaymentMethodID{"PM-card-secondary", "PM-bank-transfer", "PM-payment-slip"}
	wantBackoff := []time.Duration{24 * time.Hour, 48 * time.Hour, 96 * time.Hour}
	for i := range wantMethods {
		d := c.RecordFailure()
		if d.Kind != DecisionRetry {
			t.Fatalf("attempt %d: Kind = %v, want Retry", i, d.Kind)
		}
		if d.Method != wantMethods[i] {
			t.Errorf("attempt %d: Method = %q, want %q", i, d.Method, wantMethods[i])
		}
		if d.Backoff != wantBackoff[i] {
			t.Errorf("attempt %d: Backoff = %s, want %s", i, d.Backoff, wantBackoff[i])
		}
	}
}

// 全手段を試し尽くしたら（貸倒無効の戦略では）エスカレーションし、手順を添える。
func TestRecordFailure_ExhaustionEscalates(t *testing.T) {
	c := NewCase("CASE-1", "INV-1", shared.JPY(3000), DefaultStrategy())
	c.Start()

	var last Decision
	for i := 0; i < len(DefaultStrategy().MethodFallback); i++ {
		last = c.RecordFailure()
	}
	if last.Kind != DecisionEscalate {
		t.Fatalf("Kind = %v, want Escalate", last.Kind)
	}
	if c.Status != StatusEscalated {
		t.Errorf("Status = %q, want %q", c.Status, StatusEscalated)
	}
	want := []EscalationAction{ActionNotify, ActionSuspend, ActionRequestCancel}
	if len(last.Actions) != len(want) {
		t.Fatalf("Actions = %v, want %v", last.Actions, want)
	}
	for i, a := range want {
		if last.Actions[i] != a {
			t.Errorf("Actions[%d] = %q, want %q", i, last.Actions[i], a)
		}
	}
}

// MaxAttempts は手段数より少ない上限を効かせる。
func TestRecordFailure_MaxAttemptsCapsRetries(t *testing.T) {
	strat := DefaultStrategy()
	strat.Retry.MaxAttempts = 2 // 4 手段あっても 2 回でエスカレーション
	c := NewCase("CASE-1", "INV-1", shared.JPY(3000), strat)
	c.Start()

	if d := c.RecordFailure(); d.Kind != DecisionRetry {
		t.Fatalf("1 回目失敗後: Kind = %v, want Retry", d.Kind)
	}
	if d := c.RecordFailure(); d.Kind != DecisionEscalate {
		t.Errorf("2 回目失敗後: Kind = %v, want Escalate（上限到達）", d.Kind)
	}
}

// 少額債権は手段が尽きると貸倒になる。
func TestRecordFailure_WriteOffForMinorBalance(t *testing.T) {
	strat := DefaultStrategy()
	strat.MethodFallback = []shared.PaymentMethodID{"PM-card-primary"}
	strat.WriteOff = WriteOffRule{Enabled: true, MaxAmount: shared.JPY(1000)}
	c := NewCase("CASE-1", "INV-1", shared.JPY(500), strat)
	c.Start()

	d := c.RecordFailure()
	if d.Kind != DecisionWriteOff {
		t.Fatalf("Kind = %v, want WriteOff", d.Kind)
	}
	if c.Status != StatusWrittenOff {
		t.Errorf("Status = %q, want %q", c.Status, StatusWrittenOff)
	}
}

// 貸倒の閾値を超える金額は、貸倒ルールが有効でもエスカレーションへ回す。
func TestRecordFailure_WriteOffSkippedForLargeBalance(t *testing.T) {
	strat := DefaultStrategy()
	strat.MethodFallback = []shared.PaymentMethodID{"PM-card-primary"}
	strat.WriteOff = WriteOffRule{Enabled: true, MaxAmount: shared.JPY(1000)}
	c := NewCase("CASE-1", "INV-1", shared.JPY(50000), strat)
	c.Start()

	if d := c.RecordFailure(); d.Kind != DecisionEscalate {
		t.Errorf("Kind = %v, want Escalate（高額は貸倒対象外）", d.Kind)
	}
}

func TestMarkRecovered(t *testing.T) {
	c := NewCase("CASE-1", "INV-1", shared.JPY(3000), DefaultStrategy())
	if !c.MarkRecovered() {
		t.Fatal("MarkRecovered = false, want true")
	}
	if c.Status != StatusRecovered {
		t.Errorf("Status = %q, want %q", c.Status, StatusRecovered)
	}
}

// 終了済みの案件は再度回収完了にできない（冪等・多重消込防止）。
func TestMarkRecovered_NoOpAfterEscalation(t *testing.T) {
	strat := DefaultStrategy()
	strat.MethodFallback = []shared.PaymentMethodID{"PM-card-primary"}
	c := NewCase("CASE-1", "INV-1", shared.JPY(3000), strat)
	c.Start()
	c.RecordFailure() // 手段が尽きてエスカレーション

	if c.MarkRecovered() {
		t.Error("エスカレーション後に MarkRecovered = true, want false")
	}
	if c.Status != StatusEscalated {
		t.Errorf("Status = %q, want %q", c.Status, StatusEscalated)
	}
}

// 終了済み案件への RecordFailure は NoOp を返し、状態を変えない（誤遷移・再発行の防止）。
func TestRecordFailure_NoOpWhenClosed(t *testing.T) {
	strat := DefaultStrategy()
	strat.MethodFallback = []shared.PaymentMethodID{"PM-card-primary"}
	c := NewCase("CASE-1", "INV-1", shared.JPY(3000), strat)
	c.Start()
	c.RecordFailure() // 手段が尽きてエスカレーション

	d := c.RecordFailure()
	if d.Kind != DecisionNoOp {
		t.Errorf("Kind = %v, want NoOp", d.Kind)
	}
	if c.Status != StatusEscalated {
		t.Errorf("Status = %q, want %q", c.Status, StatusEscalated)
	}
}

func TestRetryPolicy_BackoffFixedInterval(t *testing.T) {
	p := RetryPolicy{BaseBackoff: time.Hour, Multiplier: 1} // 固定間隔
	for attempt, want := range map[int]time.Duration{0: 0, 1: time.Hour, 3: time.Hour} {
		if got := p.Backoff(attempt); got != want {
			t.Errorf("Backoff(%d) = %s, want %s", attempt, got, want)
		}
	}
}
