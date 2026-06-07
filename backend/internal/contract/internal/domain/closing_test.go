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

// 31 日締めを 30 日までの月（4 月）に適用すると月末（30 日）に丸まる。
func TestClosingPolicy_MonthEndIn30DayMonth(t *testing.T) {
	p := MonthEndClosing()
	got := p.ClosingDate(2026, time.April, time.UTC)
	want := time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("4 月末締め = %v, want %v", got, want)
	}
}

// うるう年の 2 月末は 29 日。
func TestClosingPolicy_LeapYearFebruary(t *testing.T) {
	p := MonthEndClosing()
	got := p.ClosingDate(2024, time.February, time.UTC)
	want := time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("2024 年 2 月末締め = %v, want %v", got, want)
	}
}

func TestClosingPolicy_FixedDay(t *testing.T) {
	p, err := NewClosingPolicy(15)
	if err != nil {
		t.Fatalf("NewClosingPolicy: %v", err)
	}
	got := p.ClosingDate(2026, time.June, time.UTC)
	want := time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("15 日締め = %v, want %v", got, want)
	}
}

// UTC 以外のタイムゾーン（JST）でも指定ロケーションの暦日で締め日を返す。
func TestClosingPolicy_NonUTCTimezone(t *testing.T) {
	jst := time.FixedZone("JST", 9*3600)
	p, err := NewClosingPolicy(15)
	if err != nil {
		t.Fatalf("NewClosingPolicy: %v", err)
	}
	got := p.ClosingDate(2026, time.June, jst)
	want := time.Date(2026, time.June, 15, 0, 0, 0, 0, jst)
	if !got.Equal(want) {
		t.Errorf("JST 15 日締め = %v, want %v", got, want)
	}
}

func TestNewClosingPolicy_RejectsOutOfRange(t *testing.T) {
	for _, day := range []int{0, -1, 32} {
		if _, err := NewClosingPolicy(day); err != ErrInvalidClosingDay {
			t.Errorf("day=%d は ErrInvalidClosingDay: got %v", day, err)
		}
	}
}
