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
