package creditnote_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/kato0373i/subscope/backend/internal/creditnote"
	"github.com/kato0373i/subscope/backend/internal/platform/eventbus"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// プラン変更の差額が負（ダウングレード返金）のとき赤伝を発行する。
func TestService_PlanDowngradeIssuesCreditNote(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = creditnote.NewService(bus)

	var issued *events.CreditNoteIssued
	bus.Subscribe(events.NameCreditNoteIssued, func(_ context.Context, e shared.Event) error {
		ev := e.(events.CreditNoteIssued)
		issued = &ev
		return nil
	})

	// 差額 -1000（返金）。
	mustPublish(t, bus, events.PlanChanged{
		ContractID:    "CT-1",
		OldPlanID:     "PLAN-2",
		NewPlanID:     "PLAN-1",
		NetAdjustment: shared.Money{Amount: -1000, Currency: "JPY"},
	})

	if issued == nil {
		t.Fatal("CreditNoteIssued が発行されなかった")
	}
	if issued.Amount.Amount != 1000 {
		t.Errorf("返金額 = %d, want 1000（差額の絶対値）", issued.Amount.Amount)
	}
}

// 追加請求（差額が正）では赤伝を発行しない。
func TestService_UpgradeDoesNotIssueCreditNote(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = creditnote.NewService(bus)

	var count int
	bus.Subscribe(events.NameCreditNoteIssued, func(context.Context, shared.Event) error { count++; return nil })

	mustPublish(t, bus, events.PlanChanged{
		ContractID:    "CT-1",
		NetAdjustment: shared.Money{Amount: 1000, Currency: "JPY"},
	})

	if count != 0 {
		t.Errorf("CreditNoteIssued = %d, want 0（追加請求は返金ではない）", count)
	}
}

// 手動発行（解約返金など）も CreditNoteIssued を発行し、適用できる。
func TestService_ManualIssueAndApply(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := creditnote.NewService(bus)

	var count int
	bus.Subscribe(events.NameCreditNoteIssued, func(context.Context, shared.Event) error { count++; return nil })

	id, err := s.Issue(context.Background(), "CT-1", shared.JPY(3000), "cancellation")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if count != 1 {
		t.Errorf("CreditNoteIssued = %d, want 1", count)
	}
	if err := s.Apply(id); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

func TestService_RejectsInvalidAmount(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := creditnote.NewService(bus)
	if _, err := s.Issue(context.Background(), "CT-1", shared.JPY(0), "x"); err == nil {
		t.Error("0 円の発行はエラーになるべき")
	}
}

// 差額が math.MinInt64 のとき、符号反転のオーバーフローを検出してエラーを返す。
func TestService_MinInt64OverflowDetected(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = creditnote.NewService(bus)

	var count int
	bus.Subscribe(events.NameCreditNoteIssued, func(context.Context, shared.Event) error { count++; return nil })

	err := bus.Publish(context.Background(), events.PlanChanged{
		ContractID:    "CT-1",
		NetAdjustment: shared.Money{Amount: math.MinInt64, Currency: "JPY"},
	})
	if !errors.Is(err, creditnote.ErrAmountOverflow) {
		t.Errorf("math.MinInt64 のオーバーフローは ErrAmountOverflow: got %v", err)
	}
	if count != 0 {
		t.Errorf("CreditNoteIssued = %d, want 0（オーバーフロー時は発行しない）", count)
	}
}

func mustPublish(t *testing.T, bus shared.EventBus, e shared.Event) {
	t.Helper()
	if err := bus.Publish(context.Background(), e); err != nil {
		t.Fatalf("Publish(%s): %v", e.EventName(), err)
	}
}
