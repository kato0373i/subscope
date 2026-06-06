package billing_test

import (
	"context"
	"testing"

	"github.com/kato0373i/subscope/backend/internal/billing"
	"github.com/kato0373i/subscope/backend/internal/platform/eventbus"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// BillingDue を受けて InvoiceIssued を発行し、その額・請求先が伝播することを固定する。
func TestService_BillingDueIssuesInvoice(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = billing.NewService(bus)

	var got *events.InvoiceIssued
	bus.Subscribe(events.NameInvoiceIssued, func(_ context.Context, e shared.Event) error {
		ev := e.(events.InvoiceIssued)
		got = &ev
		return nil
	})

	err := bus.Publish(context.Background(), events.BillingDue{
		ContractID:       "CT-1",
		BillingAccountID: "BA-1",
		Amount:           shared.JPY(3000),
		Period:           "2026-06",
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if got == nil {
		t.Fatal("InvoiceIssued が発行されなかった")
	}
	if got.Amount.Amount != 3000 {
		t.Errorf("Amount = %v, want 3000", got.Amount)
	}
	if got.BillingAccountID != "BA-1" {
		t.Errorf("BillingAccountID = %q, want BA-1", got.BillingAccountID)
	}
}
