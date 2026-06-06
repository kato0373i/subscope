package domain

import (
	"testing"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

func TestNew_StartsActiveWithBothReferences(t *testing.T) {
	c := New("CT-1", "MEM-1", "BA-1", shared.JPY(3000))

	if c.Status != StatusActive {
		t.Errorf("Status = %q, want %q", c.Status, StatusActive)
	}
	// 会員 ≠ 支払者：契約は両方を参照する。
	if c.MemberID != "MEM-1" {
		t.Errorf("MemberID = %q, want MEM-1", c.MemberID)
	}
	if c.BillingAccountID != "BA-1" {
		t.Errorf("BillingAccountID = %q, want BA-1", c.BillingAccountID)
	}
	if c.MonthlyFee.Amount != 3000 {
		t.Errorf("MonthlyFee = %v, want 3000", c.MonthlyFee)
	}
}
