package metrics_test

import (
	"context"
	"testing"

	"github.com/kato0373i/subscope/backend/internal/metrics"
	"github.com/kato0373i/subscope/backend/internal/platform/eventbus"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

func TestService_ProjectsFromEvents(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := metrics.NewService(bus)

	mustPublish(t, bus, events.ContractActivated{ContractID: "CT-1"})
	mustPublish(t, bus, events.ContractActivated{ContractID: "CT-2"})
	mustPublish(t, bus, events.ContractCancelled{ContractID: "CT-1"})
	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})
	mustPublish(t, bus, events.InvoicePaid{InvoiceID: "INV-1"})
	mustPublish(t, bus, events.CollectionRecovered{CaseID: "CASE-1", InvoiceID: "INV-9", Amount: shared.JPY(1200)})
	mustPublish(t, bus, events.CreditNoteIssued{CreditNoteID: "CN-1", ContractID: "CT-2", Amount: shared.JPY(800), Reason: "refund"})

	snap := s.Snapshot()
	if snap.ActiveContracts != 1 {
		t.Errorf("ActiveContracts = %d, want 1", snap.ActiveContracts)
	}
	if snap.ChurnedContracts != 1 {
		t.Errorf("ChurnedContracts = %d, want 1", snap.ChurnedContracts)
	}
	if snap.BilledTotal.Amount != 3000 {
		t.Errorf("BilledTotal = %d, want 3000", snap.BilledTotal.Amount)
	}
	if snap.InvoicesPaid != 1 {
		t.Errorf("InvoicesPaid = %d, want 1", snap.InvoicesPaid)
	}
	if snap.RecoveredTotal.Amount != 1200 {
		t.Errorf("RecoveredTotal = %d, want 1200", snap.RecoveredTotal.Amount)
	}
	if snap.RefundTotal.Amount != 800 {
		t.Errorf("RefundTotal = %d, want 800", snap.RefundTotal.Amount)
	}
}

// Snapshot は値コピーを返し、以降のイベントが取得済みスナップショットに波及しない。
func TestService_SnapshotIsPointInTime(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := metrics.NewService(bus)

	mustPublish(t, bus, events.ContractActivated{ContractID: "CT-1"})
	before := s.Snapshot()
	mustPublish(t, bus, events.ContractActivated{ContractID: "CT-2"})

	if before.ActiveContracts != 1 {
		t.Errorf("取得済みスナップショット = %d, want 1（不変）", before.ActiveContracts)
	}
	if s.Snapshot().ActiveContracts != 2 {
		t.Errorf("最新スナップショット = %d, want 2", s.Snapshot().ActiveContracts)
	}
}

func mustPublish(t *testing.T, bus shared.EventBus, e shared.Event) {
	t.Helper()
	if err := bus.Publish(context.Background(), e); err != nil {
		t.Fatalf("Publish(%s): %v", e.EventName(), err)
	}
}
