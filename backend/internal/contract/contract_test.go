package contract_test

import (
	"context"
	"testing"
	"time"

	"github.com/kato0373i/subscope/backend/internal/contract"
	"github.com/kato0373i/subscope/backend/internal/platform/eventbus"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// プラン変更で PlanChanged を発行し、日割り調整明細を返す。
func TestService_ChangePlanEmitsPlanChanged(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := contract.NewService(bus)

	var ev *events.PlanChanged
	bus.Subscribe(events.NamePlanChanged, func(_ context.Context, e shared.Event) error {
		p := e.(events.PlanChanged)
		ev = &p
		return nil
	})

	s.RegisterContract("CT-1", "MEM-1", "BA-1", shared.JPY(3000))
	adj, err := s.ChangePlan(context.Background(), "CT-1", "PLAN-2", shared.JPY(5000), time.Now())
	if err != nil {
		t.Fatalf("ChangePlan: %v", err)
	}
	if ev == nil {
		t.Fatal("PlanChanged が発行されなかった")
	}
	if ev.NewPlanID != "PLAN-2" {
		t.Errorf("NewPlanID = %q, want PLAN-2", ev.NewPlanID)
	}
	if ev.NetAdjustment != adj.Net {
		t.Errorf("イベントの NetAdjustment = %v, want %v", ev.NetAdjustment, adj.Net)
	}
}

// トライアル契約を有効化すると ContractActivated を発行する。
func TestService_ActivateEmitsEvent(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := contract.NewService(bus)

	var activated int
	bus.Subscribe(events.NameContractActivated, func(context.Context, shared.Event) error { activated++; return nil })

	s.RegisterTrial("CT-1", "MEM-1", "BA-1", shared.JPY(3000), 14)
	if err := s.Activate(context.Background(), "CT-1"); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if activated != 1 {
		t.Errorf("ContractActivated = %d, want 1", activated)
	}
}

// past_due を経て Suspend すると ContractSuspended を発行する。
func TestService_SuspendEmitsEvent(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := contract.NewService(bus)

	var suspended int
	bus.Subscribe(events.NameContractSuspended, func(context.Context, shared.Event) error { suspended++; return nil })

	s.RegisterContract("CT-1", "MEM-1", "BA-1", shared.JPY(3000))
	if err := s.MarkPastDue("CT-1"); err != nil {
		t.Fatalf("MarkPastDue: %v", err)
	}
	if err := s.Suspend(context.Background(), "CT-1"); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if suspended != 1 {
		t.Errorf("ContractSuspended = %d, want 1", suspended)
	}
}

// 解約すると ContractCancelled を発行する。
func TestService_CancelEmitsEvent(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := contract.NewService(bus)

	var cancelled int
	bus.Subscribe(events.NameContractCancelled, func(context.Context, shared.Event) error { cancelled++; return nil })

	s.RegisterContract("CT-1", "MEM-1", "BA-1", shared.JPY(3000))
	if err := s.Cancel(context.Background(), "CT-1"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if cancelled != 1 {
		t.Errorf("ContractCancelled = %d, want 1", cancelled)
	}
}

// 未登録の契約への操作は ErrNotFound。
func TestService_NotFound(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := contract.NewService(bus)
	if err := s.Cancel(context.Background(), "CT-X"); err != contract.ErrNotFound {
		t.Errorf("未登録の Cancel は ErrNotFound: got %v", err)
	}
}
