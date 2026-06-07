package notification_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kato0373i/subscope/backend/internal/notification"
	"github.com/kato0373i/subscope/backend/internal/platform/eventbus"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// 督促ステップ（DunningStepTriggered）を受けて送信し、NotificationSent を発行する。
func TestService_SendsOnDunningStep(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = notification.NewService(bus)

	var sent *events.NotificationSent
	bus.Subscribe(events.NameNotificationSent, func(_ context.Context, e shared.Event) error {
		ev := e.(events.NotificationSent)
		sent = &ev
		return nil
	})

	mustPublish(t, bus, events.DunningStepTriggered{
		CampaignID: "DUN-1", InvoiceID: "INV-1", Account: "BA-1", Channel: "email", StepNumber: 1,
	})

	if sent == nil {
		t.Fatal("NotificationSent が発行されなかった")
	}
	if sent.Channel != "email" || sent.InvoiceID != "INV-1" {
		t.Errorf("NotificationSent = %+v, want channel=email invoice=INV-1", *sent)
	}
}

// 送信が失敗した場合は NotificationSent を発行しない。
func TestService_DoesNotEmitOnSendFailure(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = notification.NewServiceWithSender(bus, failingSender{})

	var sent int
	bus.Subscribe(events.NameNotificationSent, func(context.Context, shared.Event) error { sent++; return nil })

	// 送信失敗はハンドラのエラーとして返るため、Publish もエラーになる。
	_ = bus.Publish(context.Background(), events.DunningStepTriggered{
		CampaignID: "DUN-1", InvoiceID: "INV-1", Account: "BA-1", Channel: "sms", StepNumber: 2,
	})

	if sent != 0 {
		t.Errorf("NotificationSent = %d, want 0（送信失敗時は発行しない）", sent)
	}
}

type failingSender struct{}

func (failingSender) Send(context.Context, *notification.Notification) error {
	return errors.New("送信失敗")
}

func mustPublish(t *testing.T, bus shared.EventBus, e shared.Event) {
	t.Helper()
	if err := bus.Publish(context.Background(), e); err != nil {
		t.Fatalf("Publish(%s): %v", e.EventName(), err)
	}
}
