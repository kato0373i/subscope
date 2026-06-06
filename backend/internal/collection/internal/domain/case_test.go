package domain

import (
	"testing"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

func TestNewCase_StartsInProgress(t *testing.T) {
	c := NewCase("CASE-1", "INV-1", shared.JPY(3000))
	if c.Status != StatusInProgress {
		t.Errorf("Status = %q, want %q", c.Status, StatusInProgress)
	}
}

// 戦略の優先順位どおりに手段を返し、失敗ごとに次の手段へ進む。
func TestNextMethod_FollowsFallbackOrder(t *testing.T) {
	c := NewCase("CASE-1", "INV-1", shared.JPY(3000))
	want := []shared.PaymentMethodID{
		"PM-card-primary",
		"PM-card-secondary",
		"PM-bank-transfer",
		"PM-payment-slip",
	}
	for i, w := range want {
		got, ok := c.NextMethod()
		if !ok {
			t.Fatalf("手段 %d 番目で尽きた", i)
		}
		if got != w {
			t.Errorf("attempt %d: NextMethod = %q, want %q", i, got, w)
		}
		c.RecordFailure()
	}
}

// 全手段を試し尽くしたら escalated になり false を返す。
func TestNextMethod_ExhaustionEscalates(t *testing.T) {
	c := NewCase("CASE-1", "INV-1", shared.JPY(3000))
	for {
		if _, ok := c.NextMethod(); !ok {
			break
		}
		c.RecordFailure()
	}
	if c.Status != StatusEscalated {
		t.Errorf("Status = %q, want %q", c.Status, StatusEscalated)
	}
}

func TestMarkRecovered(t *testing.T) {
	c := NewCase("CASE-1", "INV-1", shared.JPY(3000))
	c.MarkRecovered()
	if c.Status != StatusRecovered {
		t.Errorf("Status = %q, want %q", c.Status, StatusRecovered)
	}
}
