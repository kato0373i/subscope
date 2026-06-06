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

	// 戦略は 4 手段。各手段の失敗を順に通知すると最後にエスカレーションする。
	failAllMethods(t, bus, "INV-1")

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
	failAllMethods(t, bus, "INV-1")

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
	mustPublish(t, bus, events.PaymentFailed{InvoiceID: "INV-minor", PaymentMethodID: "PM-card-primary", Reason: "insufficient_funds"})

	// 既定の請求先：4 手段失敗でエスカレーション（貸倒にならない）。
	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-std", BillingAccountID: "BA-std", Amount: shared.JPY(3000)})
	failAllMethods(t, bus, "INV-std")

	if writtenOff != 1 {
		t.Errorf("CollectionWrittenOff = %d, want 1", writtenOff)
	}
	if escalated != 1 {
		t.Errorf("CollectionEscalated = %d, want 1", escalated)
	}
}

// 既に切替済みの手段に対する遅延/重複 PaymentFailed は無視し、手段を進めない。
func TestService_StalePaymentFailedIgnored(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = collection.NewService(bus)

	var charges, escalations int
	bus.Subscribe(events.NameChargeRequested, func(context.Context, shared.Event) error { charges++; return nil })
	bus.Subscribe(events.NameCollectionEscalated, func(context.Context, shared.Event) error { escalations++; return nil })

	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})
	// 1 手段目（PM-card-primary）失敗 → PM-card-secondary へ切替（charge 2 回目）。
	mustPublish(t, bus, events.PaymentFailed{InvoiceID: "INV-1", PaymentMethodID: "PM-card-primary", Reason: "x"})
	// 同じ PM-card-primary の遅延/重複失敗が再到着 → 既に切替済みなので無視。
	mustPublish(t, bus, events.PaymentFailed{InvoiceID: "INV-1", PaymentMethodID: "PM-card-primary", Reason: "x"})

	if charges != 2 {
		t.Errorf("ChargeRequested = %d, want 2（重複失敗は手段を進めない）", charges)
	}
	if escalations != 0 {
		t.Errorf("CollectionEscalated = %d, want 0", escalations)
	}
}

// 回収完了後に遅延 PaymentFailed が来ても CollectionEscalated を再発行しない。
func TestService_NoEscalationAfterRecovered(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = collection.NewService(bus)

	var escalations int
	bus.Subscribe(events.NameCollectionEscalated, func(context.Context, shared.Event) error { escalations++; return nil })

	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})
	mustPublish(t, bus, events.InvoicePaid{InvoiceID: "INV-1"}) // 回収完了
	// 遅延した失敗（現在手段に一致）が到着しても終了済みなので何もしない。
	mustPublish(t, bus, events.PaymentFailed{InvoiceID: "INV-1", PaymentMethodID: "PM-card-primary", Reason: "x"})

	if escalations != 0 {
		t.Errorf("CollectionEscalated = %d, want 0（回収済み案件は再エスカレーションしない）", escalations)
	}
}

// defaultFallback は既定戦略の手段フォールバック順。
var defaultFallback = []shared.PaymentMethodID{
	"PM-card-primary", "PM-card-secondary", "PM-bank-transfer", "PM-payment-slip",
}

// failAllMethods は既定戦略の全手段に対し、フォールバック順に PaymentFailed を発行する。
func failAllMethods(t *testing.T, bus shared.EventBus, inv shared.InvoiceID) {
	t.Helper()
	for _, m := range defaultFallback {
		mustPublish(t, bus, events.PaymentFailed{InvoiceID: inv, PaymentMethodID: m, Reason: "insufficient_funds"})
	}
}

func mustPublish(t *testing.T, bus shared.EventBus, e shared.Event) {
	t.Helper()
	if err := bus.Publish(context.Background(), e); err != nil {
		t.Fatalf("Publish(%s): %v", e.EventName(), err)
	}
}
