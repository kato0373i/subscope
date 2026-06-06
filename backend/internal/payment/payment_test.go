package payment_test

import (
	"context"
	"testing"

	"github.com/kato0373i/subscope/backend/internal/payment"
	"github.com/kato0373i/subscope/backend/internal/platform/eventbus"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// 副カード（PM-card-secondary）は成功を模擬し、PaymentSucceeded を発行する。
func TestService_ChargeSucceeds(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = payment.NewService(bus)

	var succeeded, failed int
	bus.Subscribe(events.NamePaymentSucceeded, func(context.Context, shared.Event) error { succeeded++; return nil })
	bus.Subscribe(events.NamePaymentFailed, func(context.Context, shared.Event) error { failed++; return nil })

	err := bus.Publish(context.Background(), events.ChargeRequested{
		InvoiceID:       "INV-1",
		PaymentMethodID: "PM-card-secondary",
		Amount:          shared.JPY(3000),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if succeeded != 1 || failed != 0 {
		t.Errorf("succeeded=%d failed=%d, want 1/0", succeeded, failed)
	}
}

// 主カードはゲートウェイ失敗を模擬し、PaymentFailed を発行する。
func TestService_ChargeFails(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = payment.NewService(bus)

	var got *events.PaymentFailed
	bus.Subscribe(events.NamePaymentFailed, func(_ context.Context, e shared.Event) error {
		ev := e.(events.PaymentFailed)
		got = &ev
		return nil
	})

	err := bus.Publish(context.Background(), events.ChargeRequested{
		InvoiceID:       "INV-1",
		PaymentMethodID: "PM-card-primary",
		Amount:          shared.JPY(3000),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if got == nil {
		t.Fatal("PaymentFailed が発行されなかった")
	}
	if got.PaymentMethodID != "PM-card-primary" {
		t.Errorf("PaymentMethodID = %q, want PM-card-primary", got.PaymentMethodID)
	}
	if got.Reason == "" {
		t.Error("FailureReason が空")
	}
}

// 同一の課金要求（冪等キー）が再送されても、決済は一度だけ実行される（二重決済しない）。
func TestService_ChargeIsIdempotent(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = payment.NewService(bus)

	var succeeded int
	bus.Subscribe(events.NamePaymentSucceeded, func(context.Context, shared.Event) error { succeeded++; return nil })

	charge := events.ChargeRequested{
		InvoiceID:       "INV-1",
		PaymentMethodID: "PM-card-secondary",
		Amount:          shared.JPY(3000),
		IdempotencyKey:  "charge:INV-1:0",
	}
	// PSP webhook 二重通知を模擬し、同じ課金要求を 2 回発行する。
	if err := bus.Publish(context.Background(), charge); err != nil {
		t.Fatalf("Publish#1: %v", err)
	}
	if err := bus.Publish(context.Background(), charge); err != nil {
		t.Fatalf("Publish#2: %v", err)
	}

	if succeeded != 1 {
		t.Errorf("PaymentSucceeded = %d, want 1（冪等キーで二重決済を抑止）", succeeded)
	}
}
