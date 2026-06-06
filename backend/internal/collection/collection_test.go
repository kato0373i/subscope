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

// エスカレーション時は戦略のエスカレーション手順をイベントに載せる。
func TestService_EscalationCarriesPlannedActions(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = collection.NewService(bus)

	var esc *events.CollectionEscalated
	bus.Subscribe(events.NameCollectionEscalated, func(_ context.Context, e shared.Event) error {
		ev := e.(events.CollectionEscalated)
		esc = &ev
		return nil
	})

	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})
	for i := 0; i < 4; i++ {
		mustPublish(t, bus, events.PaymentFailed{InvoiceID: "INV-1", Reason: "insufficient_funds"})
	}

	if esc == nil {
		t.Fatal("CollectionEscalated が発行されなかった")
	}
	want := []string{"notify", "suspend", "request_cancel"}
	if len(esc.PlannedActions) != len(want) {
		t.Fatalf("PlannedActions = %v, want %v", esc.PlannedActions, want)
	}
	for i, a := range want {
		if esc.PlannedActions[i] != a {
			t.Errorf("PlannedActions[%d] = %q, want %q", i, esc.PlannedActions[i], a)
		}
	}
}

// 入金消込（InvoicePaid）を受けると CollectionRecovered を発行する。
func TestService_InvoicePaidEmitsRecovered(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = collection.NewService(bus)

	var recovered int
	bus.Subscribe(events.NameCollectionRecovered, func(context.Context, shared.Event) error { recovered++; return nil })

	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})
	mustPublish(t, bus, events.InvoicePaid{InvoiceID: "INV-1"})
	// 二重通知が来ても回収完了は一度だけ。
	mustPublish(t, bus, events.InvoicePaid{InvoiceID: "INV-1"})

	if recovered != 1 {
		t.Errorf("CollectionRecovered = %d, want 1", recovered)
	}
}

// 貸倒戦略を適用した請求先では、手段が尽きると CollectionWrittenOff を発行する。
func TestService_WriteOffStrategyEmitsWrittenOff(t *testing.T) {
	bus := eventbus.NewInMemory()
	writeOff := collection.DefaultStrategy()
	writeOff.MethodFallback = []shared.PaymentMethodID{"PM-card-primary"}
	writeOff.WriteOff = collection.WriteOffRule{Enabled: true, MaxAmount: shared.JPY(1000)}
	_ = collection.NewServiceWithStrategy(bus, func(ba shared.BillingAccountID) collection.Strategy {
		if ba == "BA-minor" {
			return writeOff
		}
		return collection.DefaultStrategy()
	})

	var writtenOff, escalated int
	bus.Subscribe(events.NameCollectionWrittenOff, func(context.Context, shared.Event) error { writtenOff++; return nil })
	bus.Subscribe(events.NameCollectionEscalated, func(context.Context, shared.Event) error { escalated++; return nil })

	// 少額の請求先：1 手段失敗で貸倒。
	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-minor", BillingAccountID: "BA-minor", Amount: shared.JPY(500)})
	mustPublish(t, bus, events.PaymentFailed{InvoiceID: "INV-minor", Reason: "insufficient_funds"})

	// 既定の請求先：4 手段失敗でエスカレーション（貸倒にならない）。
	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-std", BillingAccountID: "BA-std", Amount: shared.JPY(3000)})
	for i := 0; i < 4; i++ {
		mustPublish(t, bus, events.PaymentFailed{InvoiceID: "INV-std", Reason: "insufficient_funds"})
	}

	if writtenOff != 1 {
		t.Errorf("CollectionWrittenOff = %d, want 1", writtenOff)
	}
	if escalated != 1 {
		t.Errorf("CollectionEscalated = %d, want 1", escalated)
	}
}

func mustPublish(t *testing.T, bus shared.EventBus, e shared.Event) {
	t.Helper()
	if err := bus.Publish(context.Background(), e); err != nil {
		t.Fatalf("Publish(%s): %v", e.EventName(), err)
	}
}
