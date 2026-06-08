package webhook_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kato0373i/subscope/backend/internal/platform/eventbus"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
	"github.com/kato0373i/subscope/backend/internal/webhook"
)

// 購読しているイベントだけが、登録済みの配信先に配信される。
func TestService_DeliversSubscribedEvents(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := webhook.NewService(bus)
	s.RegisterEndpoint("WE-1", "https://example.test/hook", events.NameInvoiceIssued)

	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})
	mustPublish(t, bus, events.InvoicePaid{InvoiceID: "INV-1"}) // 非購読

	ds := s.Deliveries()
	if len(ds) != 1 {
		t.Fatalf("配信件数 = %d, want 1（購読イベントのみ）", len(ds))
	}
	if ds[0].Status != "delivered" {
		t.Errorf("Status = %q, want delivered", ds[0].Status)
	}
	if ds[0].EventName != events.NameInvoiceIssued {
		t.Errorf("EventName = %q", ds[0].EventName)
	}
}

// 常に失敗する Transport ではリトライ上限まで試行し、failed で確定する。
func TestService_RetriesThenFails(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := webhook.NewServiceWithTransport(bus, failingTransport{})
	s.RegisterEndpoint("WE-1", "https://example.test/hook", events.NameInvoicePaid)

	mustPublish(t, bus, events.InvoicePaid{InvoiceID: "INV-1"})

	ds := s.Deliveries()
	if len(ds) != 1 {
		t.Fatalf("配信件数 = %d, want 1", len(ds))
	}
	if ds[0].Status != "failed" {
		t.Errorf("Status = %q, want failed", ds[0].Status)
	}
	if ds[0].Attempts != 3 {
		t.Errorf("Attempts = %d, want 3（上限まで試行）", ds[0].Attempts)
	}
}

// 配信先未登録なら配信は発生しない。
func TestService_NoEndpointsNoDeliveries(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := webhook.NewService(bus)
	mustPublish(t, bus, events.InvoicePaid{InvoiceID: "INV-1"})
	if len(s.Deliveries()) != 0 {
		t.Errorf("配信件数 = %d, want 0", len(s.Deliveries()))
	}
}

type failingTransport struct{}

func (failingTransport) Deliver(context.Context, string, string, string) error {
	return errors.New("boom")
}

func mustPublish(t *testing.T, bus shared.EventBus, e shared.Event) {
	t.Helper()
	if err := bus.Publish(context.Background(), e); err != nil {
		t.Fatalf("Publish(%s): %v", e.EventName(), err)
	}
}
