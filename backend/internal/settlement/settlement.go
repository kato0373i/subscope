// Package settlement は入金・消込モジュールの公開 API。
// 「実際に入金された事実」を債権に適用し、Invoice の入金済み化をトリガする。
package settlement

import (
	"context"
	"fmt"
	"log"

	"github.com/kato0373i/subscope/backend/internal/settlement/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

type Service struct {
	bus         shared.EventBus
	settlements map[shared.SettlementID]*domain.Settlement
	seq         int
}

func NewService(bus shared.EventBus) *Service {
	s := &Service{bus: bus, settlements: make(map[shared.SettlementID]*domain.Settlement)}
	// クレカは決済成功＝即入金。口座振替/振込は本来ここに「銀行データ取込」経路が加わる。
	bus.Subscribe(events.NamePaymentSucceeded, s.onPaymentSucceeded)
	return s
}

func (s *Service) onPaymentSucceeded(ctx context.Context, e shared.Event) error {
	ev := e.(events.PaymentSucceeded)
	s.seq++
	st := domain.New(shared.SettlementID(fmt.Sprintf("STL-%04d", s.seq)), ev.InvoiceID, ev.Amount)
	s.settlements[st.ID] = st
	log.Printf("[settlement] 入金を消し込み settlement=%s invoice=%s amount=%s", st.ID, ev.InvoiceID, st.Amount)
	return s.bus.Publish(ctx, events.InvoicePaid{InvoiceID: ev.InvoiceID})
}
