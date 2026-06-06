package settlement_test

import (
	"context"
	"testing"

	"github.com/kato0373i/subscope/backend/internal/platform/eventbus"
	"github.com/kato0373i/subscope/backend/internal/settlement"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// クレカ即時入金（PaymentSucceeded）を受けて消込し、InvoicePaid を発行する。
func TestService_PaymentSucceededSettlesInvoice(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = settlement.NewService(bus)

	var got *events.InvoicePaid
	bus.Subscribe(events.NameInvoicePaid, func(_ context.Context, e shared.Event) error {
		ev := e.(events.InvoicePaid)
		got = &ev
		return nil
	})

	err := bus.Publish(context.Background(), events.PaymentSucceeded{
		InvoiceID:     "INV-1",
		TransactionID: "TXN-1",
		Amount:        shared.JPY(3000),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if got == nil {
		t.Fatal("InvoicePaid が発行されなかった")
	}
	if got.InvoiceID != "INV-1" {
		t.Errorf("InvoiceID = %q, want INV-1", got.InvoiceID)
	}
}

// 同一の入金（同じ TransactionID）が二重通知されても、消込は一度だけ・InvoicePaid も一度だけ。
func TestService_SettlementIsIdempotent(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = settlement.NewService(bus)

	var paid int
	bus.Subscribe(events.NameInvoicePaid, func(context.Context, shared.Event) error { paid++; return nil })

	succeeded := events.PaymentSucceeded{
		InvoiceID:     "INV-1",
		TransactionID: "TXN-1",
		Amount:        shared.JPY(3000),
	}
	// PSP の二重通知を模擬。
	if err := bus.Publish(context.Background(), succeeded); err != nil {
		t.Fatalf("Publish#1: %v", err)
	}
	if err := bus.Publish(context.Background(), succeeded); err != nil {
		t.Fatalf("Publish#2: %v", err)
	}

	if paid != 1 {
		t.Errorf("InvoicePaid = %d, want 1（二重消込を抑止）", paid)
	}
}
