package domain

import (
	"errors"
	"time"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

var (
	// ErrCurrencyMismatch は新旧プランの通貨が異なる場合に返る。
	ErrCurrencyMismatch = errors.New("通貨が一致しません")
	// ErrInvalidPeriod は請求期間が不正（日数ゼロ以下）な場合に返る。
	ErrInvalidPeriod = errors.New("請求期間が不正です")
)

// Adjustment はプラン変更時の日割り調整の明細（値オブジェクト）。
// Credit は旧プランの未使用分（返金相当）、Charge は新プランの残期間分の課金、
// Net は差額（正なら追加請求、負なら返金）。
type Adjustment struct {
	Credit        shared.Money
	Charge        shared.Money
	Net           shared.Money
	DaysRemaining int
	DaysInPeriod  int
}

// ProrationPolicy はプラン変更時の日割り計算を行うドメインサービス。
type ProrationPolicy struct{}

// Calculate は変更日 changeDate における日割り調整を計算する。
// 現在の請求周期（前回請求日〜次回請求日）の残日数に応じて、旧プランの未使用分を
// クレジットし、新プランの残期間分を課金する。端数は切り捨て。
func (ProrationPolicy) Calculate(c *Contract, newFee shared.Money, changeDate time.Time) (Adjustment, error) {
	if c.MonthlyFee.Currency != newFee.Currency {
		return Adjustment{}, ErrCurrencyMismatch
	}
	periodEnd := c.NextBillingDate
	periodStart := c.calcPrevBillingDate()

	daysInPeriod := daysBetween(periodStart, periodEnd)
	if daysInPeriod <= 0 {
		return Adjustment{}, ErrInvalidPeriod
	}
	// changeDate は現在の請求期間 [periodStart, periodEnd] 内でなければならない。
	// 期間外（前後の期間に属する）の変更を現在期間で日割りするのは誤りなので弾く。
	if changeDate.Before(periodStart) || changeDate.After(periodEnd) {
		return Adjustment{}, ErrInvalidPeriod
	}
	daysRemaining := daysBetween(changeDate, periodEnd)

	credit := prorate(c.MonthlyFee, daysRemaining, daysInPeriod)
	charge := prorate(newFee, daysRemaining, daysInPeriod)
	net := shared.Money{Amount: charge.Amount - credit.Amount, Currency: newFee.Currency}

	return Adjustment{
		Credit:        credit,
		Charge:        charge,
		Net:           net,
		DaysRemaining: daysRemaining,
		DaysInPeriod:  daysInPeriod,
	}, nil
}

// daysBetween は from から to までの暦日数を返す。
// 日付のみを UTC で正規化してから差を取ることで、DST（夏時間）切替日の 23/25 時間による誤差を排除する。
func daysBetween(from, to time.Time) int {
	fromDate := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	toDate := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	return int(toDate.Sub(fromDate).Hours() / 24)
}

// prorate は fee の daysRemaining/daysInPeriod を端数切り捨てで返す。
// 中間乗算による int64 オーバーフローを避けるため除算を先に行う（floor(fee*r/d) と等価）。
func prorate(fee shared.Money, daysRemaining, daysInPeriod int) shared.Money {
	r := int64(daysRemaining)
	d := int64(daysInPeriod)
	amount := (fee.Amount/d)*r + (fee.Amount%d)*r/d
	return shared.Money{Amount: amount, Currency: fee.Currency}
}

// calcPrevBillingDate は NextBillingDate の 1 周期前の請求日を返す（請求期間の開始日）。
func (c *Contract) calcPrevBillingDate() time.Time {
	anchor := int(c.BillingAnchor)
	n := c.NextBillingDate
	switch c.BillingCycle {
	case CycleMonthly:
		return anchorDate(n.Year(), n.Month()-1, anchor, n.Location())
	case CycleQuarterly:
		return anchorDate(n.Year(), n.Month()-3, anchor, n.Location())
	case CycleAnnual:
		return anchorDate(n.Year()-1, n.Month(), anchor, n.Location())
	default:
		return anchorDate(n.Year(), n.Month()-1, anchor, n.Location())
	}
}

// ChangePlan はプランと月額を差し替える。キャンセル済みは不可。
func (c *Contract) ChangePlan(newPlanID shared.PlanID, newFee shared.Money) error {
	if c.Status == StatusCancelled {
		return ErrAlreadyCancelled
	}
	c.PlanID = newPlanID
	c.MonthlyFee = newFee
	return nil
}
