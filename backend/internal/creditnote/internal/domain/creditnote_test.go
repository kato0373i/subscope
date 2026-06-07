package domain

import (
	"testing"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

func TestNew_StartsIssued(t *testing.T) {
	n, err := New("CN-1", "CT-1", shared.JPY(1500), "plan_downgrade")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if n.Status != StatusIssued {
		t.Errorf("Status = %q, want %q", n.Status, StatusIssued)
	}
}

func TestNew_RejectsNonPositive(t *testing.T) {
	if _, err := New("CN-1", "CT-1", shared.JPY(0), "x"); err != ErrNonPositiveAmount {
		t.Errorf("0 円は ErrNonPositiveAmount: got %v", err)
	}
	if _, err := New("CN-1", "CT-1", shared.JPY(-100), "x"); err != ErrNonPositiveAmount {
		t.Errorf("負数は ErrNonPositiveAmount: got %v", err)
	}
}

func TestApply_FromIssuedOnly(t *testing.T) {
	n, _ := New("CN-1", "CT-1", shared.JPY(1500), "x")
	if err := n.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if n.Status != StatusApplied {
		t.Errorf("Status = %q, want %q", n.Status, StatusApplied)
	}
	if err := n.Apply(); err != ErrNotIssued {
		t.Errorf("適用済みからの再適用は ErrNotIssued: got %v", err)
	}
}
