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
}

// Entries は内部スライスのコピーを返し、取得後の追記が既存の取得結果に波及しない。
func TestService_EntriesReturnsCopy(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := audit.NewService(bus)
	mustPublish(t, bus, events.InvoicePaid{InvoiceID: "INV-1"})

	got := s.Entries()
	if len(got) != 1 {
		t.Fatalf("取得件数 = %d, want 1", len(got))
	}
	// さらに記録しても、先に取得したスライスは長さ 1 のまま（独立したコピー）。
	mustPublish(t, bus, events.InvoicePaid{InvoiceID: "INV-2"})
	if len(got) != 1 {
		t.Errorf("取得済みスライス長 = %d, want 1（コピーは不変）", len(got))
	}
	if s.Len() != 2 {
		t.Errorf("記録件数 = %d, want 2", s.Len())
	}
}

func mustPublish(t *testing.T, bus shared.EventBus, e shared.Event) {
	t.Helper()
	if err := bus.Publish(context.Background(), e); err != nil {
		t.Fatalf("Publish(%s): %v", e.EventName(), err)
	}
}
