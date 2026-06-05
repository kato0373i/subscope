// Package collection は回収モジュールの公開 API。
// 未収債権に対し、戦略に従ってリトライ・決済手段の切替・エスカレーションを指揮する。
package collection

import (
	"context"
	"fmt"
	"log"

	"github.com/kato0373i/subscope/backend/internal/collection/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

type Service struct {
	bus   shared.EventBus
	cases map[shared.InvoiceID]*domain.Case
	seq   int
}

func NewService(bus shared.EventBus) *Service {
	s := &Service{bus: bus, cases: make(map[shared.InvoiceID]*domain.Case)}
	bus.Subscribe(events.NameInvoiceIssued, s.onInvoiceIssued)
	bus.Subscribe(events.NamePaymentFailed, s.onPaymentFailed)
	bus.Subscribe(events.NameInvoicePaid, s.onInvoicePaid)
	return s
}

func (s *Service) onInvoiceIssued(ctx context.Context, e shared.Event) error {
	ev := e.(events.InvoiceIssued)
	s.seq++
	c := domain.NewCase(shared.CollectionCaseID(fmt.Sprintf("CASE-%04d", s.seq)), ev.InvoiceID, ev.Amount)
	s.cases[ev.InvoiceID] = c

	method, ok := c.NextMethod()
	if !ok {
		return nil
	}
	log.Printf("[collection] 回収案件を起票 case=%s invoice=%s → 決済手段 %s で課金", c.ID, ev.InvoiceID, method)
	return s.bus.Publish(ctx, events.ChargeRequested{
		InvoiceID:       ev.InvoiceID,
		PaymentMethodID: method,
		Amount:          ev.Amount,
	})
}

func (s *Service) onPaymentFailed(ctx context.Context, e shared.Event) error {
	ev := e.(events.PaymentFailed)
	c, ok := s.cases[ev.InvoiceID]
	if !ok {
		return nil
	}
	c.RecordFailure()

	method, ok := c.NextMethod()
	if !ok {
		log.Printf("[collection] 全手段が尽きた case=%s → エスカレーション（督促/解約へ）", c.ID)
		return s.bus.Publish(ctx) // 実装では CollectionEscalated を発行
	}
	log.Printf("[collection] 決済失敗 → 戦略に従い手段を切替 case=%s 次の手段=%s", c.ID, method)
	return s.bus.Publish(ctx, events.ChargeRequested{
		InvoiceID:       c.Invoice,
		PaymentMethodID: method,
		Amount:          c.Amount,
	})
}

func (s *Service) onInvoicePaid(ctx context.Context, e shared.Event) error {
	ev := e.(events.InvoicePaid)
	if c, ok := s.cases[ev.InvoiceID]; ok {
		c.MarkRecovered()
		log.Printf("[collection] 回収完了 case=%s status=%s", c.ID, c.Status)
	}
	return nil
}
