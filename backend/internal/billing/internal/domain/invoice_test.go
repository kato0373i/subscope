package domain

import (
	"testing"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

func TestNewIssued_StartsIssued(t *testing.T) {
	inv := NewIssued("INV-1", "CT-1", "BA-1", shared.JPY(3000))

	if inv.Status != StatusIssued {
		t.Errorf("Status = %q, want %q", inv.Status, StatusIssued)
	}
	if inv.Amount.Amount != 3000 {
		t.Errorf("Amount = %v, want 3000", inv.Amount)
	}
}

func TestMarkPaid_FromIssued(t *testing.T) {
	inv := NewIssued("INV-1", "CT-1", "BA-1", shared.JPY(3000))
	inv.MarkPaid()
	if inv.Status != StatusPaid {
		t.Errorf("Status = %q, want %q", inv.Status, StatusPaid)
	}
}

// 状態は単調に進む：paid から戻れない（再度 MarkPaid を呼んでも paid のまま）。
func TestMarkPaid_IsMonotonic(t *testing.T) {
	inv := NewIssued("INV-1", "CT-1", "BA-1", shared.JPY(3000))
	inv.MarkPaid()
	inv.MarkPaid()
	if inv.Status != StatusPaid {
		t.Errorf("Status = %q, want %q", inv.Status, StatusPaid)
	}
}

// void からは paid へ進めない（issued からのみ paid へ）。
func TestMarkPaid_RejectsNonIssued(t *testing.T) {
	inv := NewIssued("INV-1", "CT-1", "BA-1", shared.JPY(3000))
	inv.Status = StatusVoid
	inv.MarkPaid()
	if inv.Status != StatusVoid {
		t.Errorf("void からは遷移しないべき: Status = %q", inv.Status)
	}
}
