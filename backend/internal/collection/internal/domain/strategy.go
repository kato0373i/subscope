package domain

import (
	"time"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

// EscalationAction は全手段が尽きたあと下流（督促・契約）へ促すアクション。
// 通知 → 利用停止 → 解約申請 の順で重くなる。collection 自身は実行せず、
// CollectionEscalated の手順として伝えるだけ（実行は dunning / contract が担う）。
type EscalationAction string

const (
	ActionNotify        EscalationAction = "notify"         // 督促通知
	ActionSuspend       EscalationAction = "suspend"        // 利用停止
	ActionRequestCancel EscalationAction = "request_cancel" // 解約申請
)

// RetryPolicy はリトライの上限と間隔を定める。
// 「いつ・何回まで」課金を試すかの方針で、手段の選択（MethodFallback）とは独立。
type RetryPolicy struct {
	// MaxAttempts は課金試行の上限（手段切替を含む総回数）。
	// 0 は「手段が尽きるまで」を意味し、MethodFallback の長さに委ねる。
	MaxAttempts int
	// BaseBackoff は初回失敗後に次の試行まで空ける間隔。0 なら間隔なし（即時）。
	BaseBackoff time.Duration
	// Multiplier は指数バックオフの倍率。<=1 なら固定間隔。
	Multiplier int
}

// Backoff は attempt 回失敗した時点で次の試行まで空けるべき間隔を返す。
// attempt は 0 始まり（=これまでの失敗回数）。0 は初回試行で待たない。
// 1 回目の失敗後は BaseBackoff、以降は Multiplier>1 で指数的に伸ばす。
func (p RetryPolicy) Backoff(attempt int) time.Duration {
	if p.BaseBackoff <= 0 || attempt <= 0 {
		return 0
	}
	d := p.BaseBackoff
	if p.Multiplier > 1 {
		for i := 1; i < attempt; i++ {
			d *= time.Duration(p.Multiplier)
		}
	}
	return d
}

// WriteOffRule は貸倒（回収を諦めて債権を落とす）の条件。
// 少額債権を延々と追わないための「諦めライン」を表現する。
type WriteOffRule struct {
	Enabled bool
	// MaxAmount 以下の少額債権のみ貸倒対象とする。ゼロ値（Amount==0）は金額条件なし＝常に対象。
	MaxAmount shared.Money
}

// ShouldWriteOff は手段が尽きた案件を貸倒にするかを判定する。
// 無効なら false（＝エスカレーションへ回す）。通貨不一致は安全側に倒して貸倒にしない。
func (r WriteOffRule) ShouldWriteOff(amount shared.Money) bool {
	if !r.Enabled {
		return false
	}
	if r.MaxAmount.Amount == 0 {
		return true
	}
	if r.MaxAmount.Currency != amount.Currency {
		return false
	}
	return amount.Amount <= r.MaxAmount.Amount
}

// Strategy は 1 回収案件に適用する戦略。組織別・プラン別に差し替えられる単位。
type Strategy struct {
	MethodFallback []shared.PaymentMethodID // 試す決済手段の優先順
	Retry          RetryPolicy              // リトライ上限・間隔
	WriteOff       WriteOffRule             // 貸倒条件
	Escalation     []EscalationAction       // エスカレーション手順
}

// methodLimit は実際に試せる手段数。MethodFallback の長さと MaxAttempts の小さい方。
func (s Strategy) methodLimit() int {
	limit := len(s.MethodFallback)
	if s.Retry.MaxAttempts > 0 && s.Retry.MaxAttempts < limit {
		limit = s.Retry.MaxAttempts
	}
	return limit
}

// DefaultStrategy はデモ用の既定戦略：カード → 別カード → 口座振替 → 払込票。
// リトライは最大で手段数まで・指数バックオフ、貸倒は無効、尽きたら通知→停止→解約の順でエスカレーション。
func DefaultStrategy() Strategy {
	return Strategy{
		MethodFallback: []shared.PaymentMethodID{
			"PM-card-primary",
			"PM-card-secondary",
			"PM-bank-transfer",
			"PM-payment-slip",
		},
		Retry: RetryPolicy{
			MaxAttempts: 0, // 手段が尽きるまで
			BaseBackoff: 24 * time.Hour,
			Multiplier:  2,
		},
		WriteOff:   WriteOffRule{Enabled: false},
		Escalation: []EscalationAction{ActionNotify, ActionSuspend, ActionRequestCancel},
	}
}
