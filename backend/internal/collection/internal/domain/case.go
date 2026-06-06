package domain

import (
	"time"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

type CaseStatus string

const (
	StatusInProgress CaseStatus = "in_progress"
	StatusRecovered  CaseStatus = "recovered"
	StatusEscalated  CaseStatus = "escalated"
	StatusWrittenOff CaseStatus = "written_off"
)

// DecisionKind は失敗後に回収案件が取る次の一手。
type DecisionKind int

const (
	DecisionRetry    DecisionKind = iota // 次の手段で再試行
	DecisionEscalate                     // 手段が尽きた → 督促・解約へエスカレーション
	DecisionWriteOff                     // 手段が尽きた → 貸倒（少額債権など）
)

// Decision は Case が下した次アクションの指示。Service はこれを見てイベントを発行する。
type Decision struct {
	Kind    DecisionKind
	Method  shared.PaymentMethodID // Retry のとき次に課金する手段
	Backoff time.Duration          // Retry のとき次試行まで空ける間隔
	Actions []EscalationAction     // Escalate のとき下流へ促す手順
	Reason  string                 // Escalate / WriteOff の理由
}

// Case は未収となった Invoice 1 件の回収案件。
// Invoice ではなく Case が決済手段（フォールバック順）と戦略を保持する点が疎結合の肝。
type Case struct {
	ID       shared.CollectionCaseID
	Invoice  shared.InvoiceID
	Amount   shared.Money
	Status   CaseStatus
	strategy Strategy
	attempt  int // これまでの失敗回数 = 次に使う手段の index
}

// NewCase は指定の戦略で回収案件を起票する。
func NewCase(id shared.CollectionCaseID, invoice shared.InvoiceID, amount shared.Money, strategy Strategy) *Case {
	return &Case{
		ID:       id,
		Invoice:  invoice,
		Amount:   amount,
		Status:   StatusInProgress,
		strategy: strategy,
	}
}

// Start は起票直後の最初の一手を決める。手段があれば最優先手段で課金（Retry）、
// 手段がゼロなら即エスカレーション／貸倒。
func (c *Case) Start() Decision {
	return c.decide()
}

// CurrentMethod は現在の試行で使うべき決済手段を返す（副作用なし）。
// 手段が尽きた、またはリトライ上限に達した場合は false。
func (c *Case) CurrentMethod() (shared.PaymentMethodID, bool) {
	if c.attempt >= c.strategy.methodLimit() {
		return "", false
	}
	return c.strategy.MethodFallback[c.attempt], true
}

// RecordFailure は 1 回の課金失敗を記録し、次の一手を決める。
// 既に終了状態（recovered/escalated/written_off）なら何もせず Escalate(理由=closed) を返す。
func (c *Case) RecordFailure() Decision {
	if c.Status != StatusInProgress {
		return Decision{Kind: DecisionEscalate, Reason: "case_already_closed"}
	}
	c.attempt++
	return c.decide()
}

// decide は現在の試行状況から次アクションを確定し、終端なら状態を遷移させる。
func (c *Case) decide() Decision {
	if m, ok := c.CurrentMethod(); ok {
		return Decision{
			Kind:    DecisionRetry,
			Method:  m,
			Backoff: c.strategy.Retry.Backoff(c.attempt),
		}
	}
	// 手段が尽きた／上限到達。少額なら貸倒、そうでなければエスカレーション。
	if c.strategy.WriteOff.ShouldWriteOff(c.Amount) {
		c.Status = StatusWrittenOff
		return Decision{Kind: DecisionWriteOff, Reason: "uncollectible_minor_balance"}
	}
	c.Status = StatusEscalated
	return Decision{Kind: DecisionEscalate, Actions: c.strategy.Escalation, Reason: "all_methods_exhausted"}
}

// Attempt は現在の試行番号（0 始まり）。冪等キーの生成に使う。
func (c *Case) Attempt() int { return c.attempt }

// MarkRecovered は入金消込により回収完了へ遷移する。in_progress 以外からは遷移しない。
func (c *Case) MarkRecovered() bool {
	if c.Status != StatusInProgress {
		return false
	}
	c.Status = StatusRecovered
	return true
}
