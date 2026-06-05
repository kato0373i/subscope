package domain

import (
	"testing"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

// 端数処理が「税区分ごとに合算してから1回」行われることを固定する。
// 明細ごとに丸めると floor(10.5)+floor(10.5)=20 になるが、
// 税区分ごとに合算（210）してから丸めると floor(21.0)=21 になる。
func TestCalculate_RoundsPerCategoryNotPerLine(t *testing.T) {
	c := NewCalculator(RoundFloor)
	b := c.Calculate([]Line{
		{Category: CategoryStandard, NetAmount: shared.JPY(105)},
		{Category: CategoryStandard, NetAmount: shared.JPY(105)},
	})

	if got := b.TotalTax(); got != 21 {
		t.Fatalf("TotalTax = %d, want 21（税区分ごとの丸め）", got)
	}
}

func TestCalculate_MultipleRates(t *testing.T) {
	c := NewCalculator(RoundFloor)
	b := c.Calculate([]Line{
		{Category: CategoryStandard, NetAmount: shared.JPY(1080)}, // 10% -> 108
		{Category: CategoryReduced, NetAmount: shared.JPY(1080)},  // 8%  -> 86 (floor 86.4)
		{Category: CategoryExempt, NetAmount: shared.JPY(500)},    // 0%  -> 0
	})

	if got, want := b.TotalNet(), int64(2660); got != want {
		t.Errorf("TotalNet = %d, want %d", got, want)
	}
	if got, want := b.TotalTax(), int64(194); got != want {
		t.Errorf("TotalTax = %d, want %d", got, want)
	}
	if got, want := b.TotalGross(), int64(2854); got != want {
		t.Errorf("TotalGross = %d, want %d", got, want)
	}
	if got := len(b.ByCategory); got != 3 {
		t.Fatalf("len(ByCategory) = %d, want 3", got)
	}
}

func TestCalculate_RoundingModes(t *testing.T) {
	// 標準税率 105 円 -> 税額 10.5 円。モードごとに結果が変わる。
	tests := []struct {
		mode RoundingMode
		want int64
	}{
		{RoundFloor, 10},
		{RoundCeil, 11},
		{RoundHalfUp, 11},
	}
	for _, tt := range tests {
		c := NewCalculator(tt.mode)
		b := c.Calculate([]Line{{Category: CategoryStandard, NetAmount: shared.JPY(105)}})
		if got := b.TotalTax(); got != tt.want {
			t.Errorf("mode=%s: TotalTax = %d, want %d", tt.mode, got, tt.want)
		}
	}
}
