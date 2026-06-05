// Package payment は決済実行モジュールの公開 API。
// PSP（決済代行）への呼び出しは本来この層の ACL に閉じ込める。
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
	txs map[shared.TransactionID]*domain.Transaction
	seq int
}

func NewService(bus shared.EventBus) *Service {
	s := &Service{bus: bus, txs: make(map[shared.TransactionID]*domain.Transaction)}
	bus.Subscribe(events.NameChargeRequested, s.onChargeRequested)
	return s
}

func (s *Service) onChargeRequested(ctx context.Context, e shared.Event) error {
	ev := e.(events.ChargeRequested)
	s.seq++
	tx := domain.NewTransaction(
		shared.TransactionID(fmt.Sprintf("TXN-%04d", s.seq)),
		ev.InvoiceID, ev.PaymentMethodID, ev.Amount,
	)
	s.txs[tx.ID] = tx

	// PSP への決済実行を模擬。本来は ACL 経由でゲートウェイを呼ぶ。
	if simulateGatewayFails(ev.PaymentMethodID) {
		tx.MarkFailed("insufficient_funds")
		log.Printf("[payment]    決済失敗 txn=%s method=%s reason=%s", tx.ID, ev.PaymentMethodID, tx.FailureReason)
		return s.bus.Publish(ctx, events.PaymentFailed{
			InvoiceID:       ev.InvoiceID,
			TransactionID:   tx.ID,
			PaymentMethodID: ev.PaymentMethodID,
			Reason:          tx.FailureReason,
		})
	}
	tx.MarkCaptured()
	log.Printf("[payment]    決済成功 txn=%s method=%s amount=%s", tx.ID, ev.PaymentMethodID, tx.Amount)
	return s.bus.Publish(ctx, events.PaymentSucceeded{
		InvoiceID:     ev.InvoiceID,
		TransactionID: tx.ID,
		Amount:        ev.Amount,
	})
}

// simulateGatewayFails はデモ用に主カードを失敗させ、回収戦略のフォールバックを発火させる。
func simulateGatewayFails(m shared.PaymentMethodID) bool {
	return m == "PM-card-primary"
}
