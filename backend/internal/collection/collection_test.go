package collection_test

import (
	"context"
	"testing"

	"github.com/kato0373i/subscope/backend/internal/collection"
	"github.com/kato0373i/subscope/backend/internal/platform/eventbus"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// 起票直後は最優先の決済手段で課金要求を出す。
func TestService_InvoiceIssuedRequestsFirstMethod(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = collection.NewService(bus)

	var got *events.ChargeRequested
	bus.Subscribe(events.NameChargeRequested, func(_ context.Context, e shared.Event) error {
		ev := e.(events.ChargeRequested)
		got = &ev
		return nil
	})

	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})

	if got == nil {
		t.Fatal("ChargeRequested が発行されなかった")
	}
	if got.PaymentMethodID != "PM-card-primary" {
		t.Errorf("最優先手段 = %q, want PM-card-primary", got.PaymentMethodID)
	}
}

// 失敗が続いて全手段を試し尽くすと CollectionEscalated を発行する。
func TestService_ExhaustingMethodsEscalates(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = collection.NewService(bus)

	var charges, escalations int
	bus.Subscribe(events.NameChargeRequested, func(context.Context, shared.Event) error { charges++; return nil })
	bus.Subscribe(events.NameCollectionEscalated, func(context.Context, shared.Event) error { escalations++; return nil })

	// 起票（1 手段目を要求）。
	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})

	// 戦略は 4 手段。失敗を繰り返すと最後にエスカレーションする。
	for i := 0; i < 4; i++ {
		mustPublish(t, bus, events.PaymentFailed{
			InvoiceID:       "INV-1",
			TransactionID:   "TXN",
			PaymentMethodID: "PM",
			Reason:          "insufficient_funds",
		})
	}

	if charges != 4 {
		t.Errorf("ChargeRequested = %d, want 4（4 手段すべて試行）", charges)
	}
	if escalations != 1 {
		t.Errorf("CollectionEscalated = %d, want 1", escalations)
	}
}

func mustPublish(t *testing.T, bus shared.EventBus, e shared.Event) {
	t.Helper()
	if err := bus.Publish(context.Background(), e); err != nil {
		t.Fatalf("Publish(%s): %v", e.EventName(), err)
	}
}
