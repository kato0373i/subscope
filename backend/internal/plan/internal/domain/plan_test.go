package domain

import (
	"testing"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

func mustPrice(t *testing.T, amount shared.Money, interval BillingInterval) Price {
	t.Helper()
	p, err := NewPrice(amount, interval)
	if err != nil {
		t.Fatalf("NewPrice: %v", err)
	}
	return p
}

func TestNewPrice_Validations(t *testing.T) {
	if _, err := NewPrice(shared.JPY(0), IntervalMonthly); err != ErrNonPositivePrice {
		t.Errorf("0 円は ErrNonPositivePrice を返すべき: got %v", err)
	}
	if _, err := NewPrice(shared.JPY(1000), "weekly"); err != ErrInvalidInterval {
		t.Errorf("不正な周期は ErrInvalidInterval を返すべき: got %v", err)
	}
}

func TestNew_RejectsEmptyName(t *testing.T) {
	if _, err := New("PLAN-1", "ORG-1", "", mustPrice(t, shared.JPY(3000), IntervalMonthly)); err != ErrEmptyName {
		t.Errorf("空のプラン名は ErrEmptyName を返すべき: got %v", err)
	}
}

// 価格改定後も、改定前に取得したスナップショットは独立して不変であること。
func TestSnapshot_IsIndependentOfLaterPriceChange(t *testing.T) {
	p, err := New("PLAN-1", "ORG-1", "月会費", mustPrice(t, shared.JPY(3000), IntervalMonthly))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	snap := p.Snapshot()

	p.ChangePrice(mustPrice(t, shared.JPY(5000), IntervalMonthly))

	if snap.Amount.Amount != 3000 {
		t.Errorf("スナップショット = %d, want 3000（改定の影響を受けない）", snap.Amount.Amount)
	}
	if p.Price().Amount.Amount != 5000 {
		t.Errorf("改定後の現在価格 = %d, want 5000", p.Price().Amount.Amount)
	}
}
