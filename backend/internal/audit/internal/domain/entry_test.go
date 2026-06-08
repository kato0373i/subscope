package domain

import (
	"testing"
	"time"
)

func TestNewEntry_IsImmutableSnapshot(t *testing.T) {
	at := time.Date(2026, time.June, 8, 10, 0, 0, 0, time.UTC)
	e := NewEntry("AUD-0001", "billing.InvoiceIssued", "{InvoiceID:INV-1}", at)

	if e.ID() != "AUD-0001" {
		t.Errorf("ID = %q, want AUD-0001", e.ID())
	}
	if e.EventName() != "billing.InvoiceIssued" {
		t.Errorf("EventName = %q", e.EventName())
	}
	if e.Detail() != "{InvoiceID:INV-1}" {
		t.Errorf("Detail = %q", e.Detail())
	}
	if !e.RecordedAt().Equal(at) {
		t.Errorf("RecordedAt = %v, want %v", e.RecordedAt(), at)
	}
}
