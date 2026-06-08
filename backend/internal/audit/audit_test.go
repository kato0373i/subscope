package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/kato0373i/subscope/backend/internal/audit"
	"github.com/kato0373i/subscope/backend/internal/platform/eventbus"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

func TestService_RecordsEveryPublishedEvent(t *testing.T) {
	bus := eventbus.NewInMemory()
	at := time.Date(2026, time.June, 8, 0, 0, 0, 0, time.UTC)
	s := audit.NewServiceWithClock(bus, func() time.Time { return at })

	ctx := context.Background()
	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})
	mustPublish(t, bus, events.InvoicePaid{InvoiceID: "INV-1"})

	if s.Len() != 2 {
		t.Fatalf("記録件数 = %d, want 2", s.Len())
	}
	entries := s.Entries()
	if entries[0].EventName() != events.NameInvoiceIssued {
		t.Errorf("entries[0].EventName = %q, want %q", entries[0].EventName(), events.NameInvoiceIssued)
	}
	if entries[1].EventName() != events.NameInvoicePaid {
		t.Errorf("entries[1].EventName = %q, want %q", entries[1].EventName(), events.NameInvoicePaid)
	}
	if !entries[0].RecordedAt().Equal(at) {
		t.Errorf("RecordedAt = %v, want %v", entries[0].RecordedAt(), at)
	}
	_ = ctx
}

// Entries は内部スライスのコピーを返し、呼び出し側からの改変が記録に波及しない。
func TestService_EntriesReturnsCopy(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := audit.NewService(bus)
	mustPublish(t, bus, events.InvoicePaid{InvoiceID: "INV-1"})

	got := s.Entries()
	got = append(got, got...) // ローカルコピーを破壊
	if s.Len() != 1 {
		t.Errorf("記録件数 = %d, want 1（内部状態は不変）", s.Len())
	}
}

func mustPublish(t *testing.T, bus shared.EventBus, e shared.Event) {
	t.Helper()
	if err := bus.Publish(context.Background(), e); err != nil {
		t.Fatalf("Publish(%s): %v", e.EventName(), err)
	}
}
