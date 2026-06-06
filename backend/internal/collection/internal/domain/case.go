package domain

import "github.com/kato0373i/subscope/backend/internal/shared"

type CaseStatus string

const (
	StatusInProgress CaseStatus = "in_progress"
	StatusRecovered  CaseStatus = "recovered"
	StatusEscalated  CaseStatus = "escalated"
)

// Case は未収となった Invoice 1 件の回収案件。
// Invoice ではなく Case が決済手段（フォールバック順）を保持する点が疎結合の肝。
type Case struct {
	ID       shared.CollectionCaseID
	Invoice  shared.InvoiceID
	Amount   shared.Money
	Status   CaseStatus
	strategy strategy
	attempt  int
}

// strategy は回収戦略。失敗時にどの決済手段へ切り替えるかの優先順位を持つ。
type strategy struct {
	methodFallback []shared.PaymentMethodID
}

// defaultStrategy はデモ用の固定戦略：カード → 別カード → 口座振替 → 払込票。
func defaultStrategy() strategy {
	return strategy{methodFallback: []shared.PaymentMethodID{
		"PM-card-primary",
		"PM-card-secondary",
		"PM-bank-transfer",
		"PM-payment-slip",
	}}
}

func NewCase(id shared.CollectionCaseID, invoice shared.InvoiceID, amount shared.Money) *Case {
	return &Case{
		ID:       id,
		Invoice:  invoice,
		Amount:   amount,
		Status:   StatusInProgress,
		strategy: defaultStrategy(),
	}
}

// NextMethod は戦略に従い次に試すべき決済手段を返す。尽きたら escalated にして false。
func (c *Case) NextMethod() (shared.PaymentMethodID, bool) {
	if c.attempt >= len(c.strategy.methodFallback) {
		c.Status = StatusEscalated
		return "", false
	}
	return c.strategy.methodFallback[c.attempt], true
}

func (c *Case) RecordFailure() { c.attempt++ }

// Attempt は現在の試行番号（0 始まり）。冪等キーの生成に使う。
func (c *Case) Attempt() int { return c.attempt }

func (c *Case) MarkRecovered() { c.Status = StatusRecovered }
