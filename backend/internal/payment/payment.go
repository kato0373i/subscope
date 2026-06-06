// Package payment は決済実行モジュールの公開 API。
// Invoice と PaymentMethod が「1 回の試行」として初めて出会う層であり、
// PSP（決済代行）への呼び出しは ACL（Gateway）に閉じ込める。
//
// クレカは同期で captured、口座振替・払込票は pending を返して後日 settlement が
// 確定させる。この非同期性を Transaction の Status で一級市民として扱う。
package payment

import (
	"context"
	"fmt"
	"log"

	"github.com/kato0373i/subscope/backend/internal/payment/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

type Service struct {
	bus shared.EventBus
	gw  Gateway
	txs map[shared.TransactionID]*domain.Transaction
	// seen は処理済みの課金要求キー。PSP の二重通知・イベント再送による二重決済を防ぐ。
	seen map[shared.IdempotencyKey]bool
	seq  int
}

// NewService は本番既定の MockGateway で Service を組み立てる。
func NewService(bus shared.EventBus) *Service {
	return NewServiceWithGateway(bus, MockGateway{})
}

// NewServiceWithGateway は PSP ゲートウェイ（ACL）を差し替え可能にする。
// テストや PSP ごとの実装切替で使う。
func NewServiceWithGateway(bus shared.EventBus, gw Gateway) *Service {
	s := &Service{
		bus:  bus,
		gw:   gw,
		txs:  make(map[shared.TransactionID]*domain.Transaction),
		seen: make(map[shared.IdempotencyKey]bool),
	}
	bus.Subscribe(events.NameChargeRequested, s.onChargeRequested)
	return s
}

func (s *Service) onChargeRequested(ctx context.Context, e shared.Event) error {
	ev := e.(events.ChargeRequested)

	// 冪等性：同一の課金要求（再送）は二重に決済しない。
	if ev.IdempotencyKey != "" {
		if s.seen[ev.IdempotencyKey] {
			log.Printf("[payment]    重複課金要求を無視 key=%s（冪等）", ev.IdempotencyKey)
			return nil
		}
		s.seen[ev.IdempotencyKey] = true
	}

	s.seq++
	tx := domain.NewTransaction(
		shared.TransactionID(fmt.Sprintf("TXN-%04d", s.seq)),
		ev.InvoiceID, ev.PaymentMethodID, ev.Amount,
	)
	s.txs[tx.ID] = tx

	// PSP への決済実行は ACL（Gateway）越しに行い、ベンダ差異・確定タイミングを閉じ込める。
	res, err := s.gw.Charge(ctx, ChargeInput{
		TransactionID:   tx.ID,
		InvoiceID:       ev.InvoiceID,
		PaymentMethodID: ev.PaymentMethodID,
		Amount:          ev.Amount,
	})
	if err != nil {
		return fmt.Errorf("payment: PSP 呼び出しに失敗 txn=%s: %w", tx.ID, err)
	}

	switch res.Outcome {
	case OutcomeFailed:
		if err := tx.MarkFailed(res.Reason); err != nil {
			return err
		}
		log.Printf("[payment]    決済失敗 txn=%s method=%s reason=%s", tx.ID, ev.PaymentMethodID, tx.FailureReason)
		return s.bus.Publish(ctx, events.PaymentFailed{
			InvoiceID:       ev.InvoiceID,
			TransactionID:   tx.ID,
			PaymentMethodID: ev.PaymentMethodID,
			Reason:          tx.FailureReason,
		})

	case OutcomePending:
		if err := tx.MarkPending(); err != nil {
			return err
		}
		log.Printf("[payment]    後日確定待ち txn=%s method=%s amount=%s（pending）", tx.ID, ev.PaymentMethodID, tx.Amount)
		return s.bus.Publish(ctx, events.PaymentPending{
			InvoiceID:       ev.InvoiceID,
			TransactionID:   tx.ID,
			PaymentMethodID: ev.PaymentMethodID,
			Amount:          ev.Amount,
		})

	default: // OutcomeCaptured
		if err := tx.MarkCaptured(); err != nil {
			return err
		}
		log.Printf("[payment]    決済成功 txn=%s method=%s amount=%s", tx.ID, ev.PaymentMethodID, tx.Amount)
		return s.bus.Publish(ctx, events.PaymentSucceeded{
			InvoiceID:     ev.InvoiceID,
			TransactionID: tx.ID,
			Amount:        ev.Amount,
		})
	}
}
