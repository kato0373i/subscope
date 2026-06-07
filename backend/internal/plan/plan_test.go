package plan_test

import (
	"testing"

	"github.com/kato0373i/subscope/backend/internal/plan"
	"github.com/kato0373i/subscope/backend/internal/shared"
)

func TestService_RegisterAndSnapshot(t *testing.T) {
	s := plan.NewService()
	price, err := plan.NewPrice(shared.JPY(3000), plan.IntervalMonthly)
	if err != nil {
		t.Fatalf("NewPrice: %v", err)
	}
	if err := s.Register("PLAN-1", "ORG-1", "月会費", price); err != nil {
		t.Fatalf("Register: %v", err)
	}

	snap, err := s.Snapshot("PLAN-1")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.Amount.Amount != 3000 || snap.Interval != plan.IntervalMonthly {
		t.Errorf("Snapshot = %+v, want 3000/monthly", snap)
	}
}

func TestService_RejectsDuplicate(t *testing.T) {
	s := plan.NewService()
	price, _ := plan.NewPrice(shared.JPY(3000), plan.IntervalMonthly)
	if err := s.Register("PLAN-1", "ORG-1", "月会費", price); err != nil {
		t.Fatalf("Register#1: %v", err)
	}
	if err := s.Register("PLAN-1", "ORG-1", "別名", price); err != plan.ErrDuplicatePlan {
		t.Errorf("重複登録は ErrDuplicatePlan を返すべき: got %v", err)
	}
}

func TestService_ChangePriceKeepsSnapshotIndependent(t *testing.T) {
	s := plan.NewService()
	p1, _ := plan.NewPrice(shared.JPY(3000), plan.IntervalMonthly)
	_ = s.Register("PLAN-1", "ORG-1", "月会費", p1)

	before, _ := s.Snapshot("PLAN-1")
	p2, _ := plan.NewPrice(shared.JPY(5000), plan.IntervalMonthly)
	if err := s.ChangePrice("PLAN-1", p2); err != nil {
		t.Fatalf("ChangePrice: %v", err)
	}
	after, _ := s.Snapshot("PLAN-1")

	if before.Amount.Amount != 3000 {
		t.Errorf("改定前スナップショット = %d, want 3000", before.Amount.Amount)
	}
	if after.Amount.Amount != 5000 {
		t.Errorf("改定後スナップショット = %d, want 5000", after.Amount.Amount)
	}
}

func TestService_SnapshotNotFound(t *testing.T) {
	s := plan.NewService()
	if _, err := s.Snapshot("PLAN-X"); err != plan.ErrNotFound {
		t.Errorf("未登録は ErrNotFound を返すべき: got %v", err)
	}
}
