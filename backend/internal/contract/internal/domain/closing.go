package domain

import "time"

// ClosingPolicy は締め日（請求期間を締める基準日）を表す値オブジェクト。
// 「月末締め翌月請求」のような締め概念を契約のスケジュール属性として持つ。
// 締め日は contract に置く（請求サイクル BillingCycle / BillingAnchor と同じく
// 契約の請求スケジュールの一部であり、billing は締められた期間を受けて請求するため）。
type ClosingPolicy struct {
	Day int // 締め日（1〜31、31 は月末扱い）
}

// MonthEndClosing は月末締めの既定ポリシー。
func MonthEndClosing() ClosingPolicy {
	return ClosingPolicy{Day: 31}
}

// ClosingDate は指定年月の締め日を返す。締め日が月末を超える場合は月末に丸める。
func (p ClosingPolicy) ClosingDate(year int, month time.Month, loc *time.Location) time.Time {
	day := p.Day
	if day <= 0 {
		day = 1
	}
	return anchorDate(year, month, day, loc)
}
