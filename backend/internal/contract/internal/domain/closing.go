package domain

import (
	"errors"
	"time"
)

// ErrInvalidClosingDay は締め日が 1〜31 の範囲外の場合に返る。
var ErrInvalidClosingDay = errors.New("締め日は 1〜31 で指定してください")

// ClosingPolicy は締め日（請求期間を締める基準日）を表す値オブジェクト。
// 「月末締め翌月請求」のような締め概念を契約のスケジュール属性として持つ。
// 締め日は contract に置く（請求サイクル BillingCycle / BillingAnchor と同じく
// 契約の請求スケジュールの一部であり、billing は締められた期間を受けて請求するため）。
// day は値オブジェクトの不変条件（1〜31、31 は月末扱い）を保証するため private。
type ClosingPolicy struct {
	day int // 締め日（1〜31、31 は月末扱い）
}

// NewClosingPolicy は締め日を検証して ClosingPolicy を生成する。
// 範囲外（1 未満・31 超）は ErrInvalidClosingDay を返す。
func NewClosingPolicy(day int) (ClosingPolicy, error) {
	if day < 1 || day > 31 {
		return ClosingPolicy{}, ErrInvalidClosingDay
	}
	return ClosingPolicy{day: day}, nil
}

// MonthEndClosing は月末締めの既定ポリシー。
func MonthEndClosing() ClosingPolicy {
	return ClosingPolicy{day: 31}
}

// Day は締め日（1〜31）を返す。ゼロ値の場合は 1 とみなす。
func (p ClosingPolicy) Day() int {
	if p.day <= 0 {
		return 1
	}
	return p.day
}

// ClosingDate は指定年月の締め日を返す。締め日が月末を超える場合は月末に丸める。
func (p ClosingPolicy) ClosingDate(year int, month time.Month, loc *time.Location) time.Time {
	return anchorDate(year, month, p.Day(), loc)
}
