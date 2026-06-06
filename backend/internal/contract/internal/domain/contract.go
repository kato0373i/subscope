// Package domain は contract モジュールの内部ドメイン。
// パスに internal を含むため、contract モジュールの外からは import できない（Go の機構で強制）。
package domain

import (
	"errors"
	"time"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

type Status string

const (
	StatusTrialing  Status = "trialing"
	StatusActive    Status = "active"
	StatusPastDue   Status = "past_due"
	StatusSuspended Status = "suspended"
	StatusCancelled Status = "cancelled"
)

// BillingCycle は請求の繰り返し周期。
type BillingCycle string

const (
	CycleMonthly   BillingCycle = "monthly"
	CycleQuarterly BillingCycle = "quarterly"
	CycleAnnual    BillingCycle = "annual"
)

// BillingAnchor は起算日（1〜31。31は月末扱い）。
type BillingAnchor int

// TrialPeriod はトライアル期間。
type TrialPeriod struct {
	Days int // トライアル日数。0 = トライアルなし。
}

var (
	ErrInvalidTransition = errors.New("無効な状態遷移です")
	ErrAlreadyCancelled  = errors.New("キャンセル済みの契約は変更できません")
)

// Contract は会員(Member)と支払者(BillingAccount)の両方を参照する契約集約。
type Contract struct {
	ID               shared.ContractID
	OrgID            shared.OrgID
	MemberID         shared.MemberID
	BillingAccountID shared.BillingAccountID
	PlanID           shared.PlanID
	Status           Status
	MonthlyFee       shared.Money
	BillingCycle     BillingCycle
	BillingAnchor    BillingAnchor
	TrialPeriod      TrialPeriod
	StartDate        time.Time
	NextBillingDate  time.Time
}

func New(id shared.ContractID, member shared.MemberID, account shared.BillingAccountID, fee shared.Money) *Contract {
	now := time.Now()
	c := &Contract{
		ID:               id,
		MemberID:         member,
		BillingAccountID: account,
		MonthlyFee:       fee,
		Status:           StatusActive,
		BillingCycle:     CycleMonthly,
		BillingAnchor:    BillingAnchor(now.Day()),
		StartDate:        now,
	}
	c.NextBillingDate = c.calcNextBillingDate(now)
	return c
}

func NewFull(
	id shared.ContractID,
	orgID shared.OrgID,
	member shared.MemberID,
	account shared.BillingAccountID,
	planID shared.PlanID,
	fee shared.Money,
	cycle BillingCycle,
	anchor BillingAnchor,
	trial TrialPeriod,
) *Contract {
	now := time.Now()
	status := StatusActive
	if trial.Days > 0 {
		status = StatusTrialing
	}
	c := &Contract{
		ID:               id,
		OrgID:            orgID,
		MemberID:         member,
		BillingAccountID: account,
		PlanID:           planID,
		Status:           status,
		MonthlyFee:       fee,
		BillingCycle:     cycle,
		BillingAnchor:    anchor,
		TrialPeriod:      trial,
		StartDate:        now,
	}
	c.NextBillingDate = c.calcNextBillingDate(now)
	return c
}

// Activate は trialing → active への遷移。トライアル終了時に呼ぶ。
func (c *Contract) Activate() error {
	if c.Status == StatusCancelled {
		return ErrAlreadyCancelled
	}
	if c.Status != StatusTrialing {
		return ErrInvalidTransition
	}
	c.Status = StatusActive
	return nil
}

// SetPastDue は active → past_due への遷移。未払いが発生したとき。
func (c *Contract) SetPastDue() error {
	if c.Status != StatusActive {
		return ErrInvalidTransition
	}
	c.Status = StatusPastDue
	return nil
}

// Suspend は past_due → suspended への遷移。一定期間未回収の場合。
func (c *Contract) Suspend() error {
	if c.Status != StatusPastDue {
		return ErrInvalidTransition
	}
	c.Status = StatusSuspended
	return nil
}

// Cancel はいずれの状態からも cancelled へ遷移する（キャンセル済みを除く）。
func (c *Contract) Cancel() error {
	if c.Status == StatusCancelled {
		return ErrAlreadyCancelled
	}
	c.Status = StatusCancelled
	return nil
}

// IsBillable は現在課金対象かどうか。トライアル中・キャンセル・停止中は対象外。
func (c *Contract) IsBillable() bool {
	return c.Status == StatusActive || c.Status == StatusPastDue
}

// AdvanceBillingDate は請求発行後に次回請求日を進める。
func (c *Contract) AdvanceBillingDate() {
	c.NextBillingDate = c.calcNextBillingDate(c.NextBillingDate)
}

// calcNextBillingDate は BillingCycle と BillingAnchor から次回請求日を算出する。
func (c *Contract) calcNextBillingDate(from time.Time) time.Time {
	anchor := int(c.BillingAnchor)
	switch c.BillingCycle {
	case CycleMonthly:
		return anchorDate(from.Year(), from.Month()+1, anchor, from.Location())
	case CycleQuarterly:
		return anchorDate(from.Year(), from.Month()+3, anchor, from.Location())
	case CycleAnnual:
		return anchorDate(from.Year()+1, from.Month(), anchor, from.Location())
	default:
		return anchorDate(from.Year(), from.Month()+1, anchor, from.Location())
	}
}

// anchorDate は指定年月の anchor 日を返す。anchor が月末を超える場合は月末に丸める。
func anchorDate(year int, month time.Month, anchor int, loc *time.Location) time.Time {
	// time.Date は month=13 を翌年1月に正規化するので overflow-safe。
	// anchor=31 の場合は翌月の0日 = 当月末日を利用して月末を求める。
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
	if anchor > lastDay {
		anchor = lastDay
	}
	return time.Date(year, month, anchor, 0, 0, 0, 0, loc)
}
