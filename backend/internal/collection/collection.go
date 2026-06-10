// Package collection は回収モジュールの公開 API。
// 未収債権に対し、戦略に従ってリトライ・決済手段の切替・エスカレーション・貸倒を指揮する。
package collection

import (
	"context"
	"fmt"
	"log"
	"sort"

	"github.com/kato0373i/subscope/backend/internal/collection/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// CaseView は回収案件の読み取り専用ビュー（外向き読みモデル）。
// invoice 単位の回収ステータスを HTTP/読み取り層へ渡す。
type CaseView struct {
	InvoiceID shared.InvoiceID
	Status    string
}

// 戦略まわりの型を公開パッケージから再エクスポートする（呼び出し側が domain を import せず設定できるように）。
type (
	Strategy         = domain.Strategy
	RetryPolicy      = domain.RetryPolicy
	WriteOffRule     = domain.WriteOffRule
	EscalationAction = domain.EscalationAction
)

// DefaultStrategy は既定の回収戦略。
func DefaultStrategy() Strategy { return domain.DefaultStrategy() }

// StrategyResolver は請求先ごとに適用する回収戦略を返す。
// 組織別・プラン別の戦略差し替えはこの関数で表現する（現状の集約は請求先粒度で解決する）。
type StrategyResolver func(shared.BillingAccountID) Strategy

type Service struct {
	bus     shared.EventBus
	resolve StrategyResolver
	cases   map[shared.InvoiceID]*domain.Case
	seq     int
}

// NewService は全請求先に既定戦略を適用するサービスを生成する。
func NewService(bus shared.EventBus) *Service {
	return NewServiceWithStrategy(bus, func(shared.BillingAccountID) Strategy {
		return domain.DefaultStrategy()
	})
}

// NewServiceWithStrategy は請求先ごとに戦略を差し替えられるサービスを生成する。
// resolver が nil の場合は既定戦略にフォールバックする。
func NewServiceWithStrategy(bus shared.EventBus, resolver StrategyResolver) *Service {
	if resolver == nil {
		resolver = func(shared.BillingAccountID) Strategy { return domain.DefaultStrategy() }
	}
	s := &Service{
		bus:     bus,
		resolve: resolver,
		cases:   make(map[shared.InvoiceID]*domain.Case),
	}
	bus.Subscribe(events.NameInvoiceIssued, s.onInvoiceIssued)
	bus.Subscribe(events.NamePaymentFailed, s.onPaymentFailed)
	bus.Subscribe(events.NameInvoicePaid, s.onInvoicePaid)
	return s
}

// ListCases は回収案件を invoice ID 昇順で返す（読み取り API 用）。
func (s *Service) ListCases() []CaseView {
	views := make([]CaseView, 0, len(s.cases))
	for _, c := range s.cases {
		views = append(views, CaseView{
			InvoiceID: c.Invoice,
			Status:    string(c.Status),
		})
	}
	sort.Slice(views, func(i, j int) bool { return views[i].InvoiceID < views[j].InvoiceID })
	return views
}

func (s *Service) onInvoiceIssued(ctx context.Context, e shared.Event) error {
	ev := e.(events.InvoiceIssued)
	s.seq++
	strat := s.resolve(ev.BillingAccountID)
	c := domain.NewCase(shared.CollectionCaseID(fmt.Sprintf("CASE-%04d", s.seq)), ev.InvoiceID, ev.Amount, strat)
	s.cases[ev.InvoiceID] = c
	log.Printf("[collection] 回収案件を起票 case=%s invoice=%s", c.ID, ev.InvoiceID)
	return s.apply(ctx, c, c.Start())
}

func (s *Service) onPaymentFailed(ctx context.Context, e shared.Event) error {
	ev := e.(events.PaymentFailed)
	c, ok := s.cases[ev.InvoiceID]
	if !ok {
		return nil
	}
	// 相関チェック：いま試行中の手段に対する失敗だけ受理する。
	// 既に切替済みの手段や終了案件への遅延・重複通知を弾き、attempt の進みすぎ・督促の誤再起動を防ぐ。
	current, ok := c.CurrentMethod()
	if !ok || ev.PaymentMethodID != current {
		log.Printf("[collection] stale/duplicate な PaymentFailed を無視 invoice=%s got=%s want=%s", ev.InvoiceID, ev.PaymentMethodID, current)
		return nil
	}
	return s.apply(ctx, c, c.RecordFailure())
}

func (s *Service) onInvoicePaid(ctx context.Context, e shared.Event) error {
	ev := e.(events.InvoicePaid)
	c, ok := s.cases[ev.InvoiceID]
	if !ok {
		return nil
	}
	if !c.MarkRecovered() {
		return nil
	}
	log.Printf("[collection] 回収完了 case=%s status=%s", c.ID, c.Status)
	return s.bus.Publish(ctx, events.CollectionRecovered{
		CaseID:    c.ID,
		InvoiceID: c.Invoice,
		Amount:    c.Amount,
	})
}

// apply は Case が下した次アクションをイベントとして発行する。
func (s *Service) apply(ctx context.Context, c *domain.Case, d domain.Decision) error {
	switch d.Kind {
	case domain.DecisionNoOp:
		return nil
	case domain.DecisionRetry:
		log.Printf("[collection] 課金要求 case=%s 手段=%s 次回間隔=%s", c.ID, d.Method, d.Backoff)
		return s.bus.Publish(ctx, events.ChargeRequested{
			InvoiceID:       c.Invoice,
			PaymentMethodID: d.Method,
			Amount:          c.Amount,
			IdempotencyKey:  chargeKey(c.Invoice, c.Attempt()),
		})
	case domain.DecisionWriteOff:
		log.Printf("[collection] 貸倒 case=%s reason=%s amount=%s", c.ID, d.Reason, c.Amount)
		return s.bus.Publish(ctx, events.CollectionWrittenOff{
			CaseID:    c.ID,
			InvoiceID: c.Invoice,
			Amount:    c.Amount,
			Reason:    d.Reason,
		})
	case domain.DecisionEscalate:
		log.Printf("[collection] 全手段が尽きた case=%s → エスカレーション %v", c.ID, d.Actions)
		return s.bus.Publish(ctx, events.CollectionEscalated{
			CaseID:         c.ID,
			InvoiceID:      c.Invoice,
			Amount:         c.Amount,
			PlannedActions: actionNames(d.Actions),
		})
	default:
		return fmt.Errorf("collection: 未知の Decision %d case=%s", d.Kind, c.ID)
	}
}

// chargeKey は課金試行ごとに安定した冪等キーを返す。
// 同一試行の ChargeRequested が再送されても payment が二重決済しないようにする。
func chargeKey(inv shared.InvoiceID, attempt int) shared.IdempotencyKey {
	return shared.IdempotencyKey(fmt.Sprintf("charge:%s:%d", inv, attempt))
}

// actionNames は型付きのエスカレーション手順をイベント契約（文字列）へ落とす。
func actionNames(actions []domain.EscalationAction) []string {
	if len(actions) == 0 {
		return nil
	}
	out := make([]string, len(actions))
	for i, a := range actions {
		out[i] = string(a)
	}
	return out
}
