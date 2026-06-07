package domain

import (
	"testing"
	"time"
)

func TestClosingPolicy_MonthEnd(t *testing.T) {
	p := MonthEndClosing()
	// 2 月末は 28 日（2026 は平年）に丸まる。
	got := p.ClosingDate(2026, time.February, time.UTC)
	want := time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("2 月末締め = %v, want %v", got, want)
	}
}

func TestClosingPolicy_FixedDay(t *testing.T) {
	p := ClosingPolicy{Day: 15}
	got := p.ClosingDate(2026, time.June, time.UTC)
	want := time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("15 日締め = %v, want %v", got, want)
	}
}
