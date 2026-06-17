// Package domain は督促モジュールの集約・状態機械を閉じ込める private ドメイン層。
// 依存は shared と標準ライブラリのみ。
package domain

import "github.com/kato0373i/subscope/backend/internal/shared"

// Channel は督促の通知チャネル。後段の重い手段ほど後ろに置く。
type Channel string

const (
	ChannelEmail  Channel = "email"  // メール
	ChannelSMS    Channel = "sms"    // SMS
	ChannelLetter Channel = "letter" // 郵送（督促状）
)

// Step は督促シーケンスの 1 ステップ。OffsetDays は起票からの経過日数（D+n）。
type Step struct {
	OffsetDays int
	Channel    Channel
}

// DefaultSequence は既定の督促シーケンス：D+0 メール → D+3 SMS → D+7 督促状。
func DefaultSequence() []Step {
	return []Step{
		{OffsetDays: 0, Channel: ChannelEmail},
		{OffsetDays: 3, Channel: ChannelSMS},
		{OffsetDays: 7, Channel: ChannelLetter},
	}
}

type CampaignStatus string

const (
	StatusActive    CampaignStatus = "active"    // 督促進行中
	StatusResolved  CampaignStatus = "resolved"  // 入金により解決（督促停止）
	StatusCompleted CampaignStatus = "completed" // 全ステップ実施済み
)

// Campaign は未収 1 件に対する督促キャンペーン。シーケンスを単調に前進させる状態機械。
type Campaign struct {
	ID        shared.DunningCampaignID
	Invoice   shared.InvoiceID
	Account   shared.BillingAccountID
	Status    CampaignStatus
	steps     []Step
	triggered int // 実施済みステップ数
}

// NewCampaign は督促キャンペーンを起票する。steps が空なら既定シーケンスを使う。
func NewCampaign(id shared.DunningCampaignID, invoice shared.InvoiceID, account shared.BillingAccountID, steps []Step) *Campaign {
	if len(steps) == 0 {
		steps = DefaultSequence()
	}
	return &Campaign{
		ID:      id,
		Invoice: invoice,
		Account: account,
		Status:  StatusActive,
		steps:   steps,
	}
}

// TriggerNext は次のステップを発火し、その Step と true を返す。
// 進行中でない、または全ステップ実施済みなら false。最後のステップを発火すると completed へ。
func (c *Campaign) TriggerNext() (Step, int, bool) {
	if c.Status != StatusActive || c.triggered >= len(c.steps) {
		return Step{}, 0, false
	}
	step := c.steps[c.triggered]
	c.triggered++
	num := c.triggered // 1 始まりのステップ番号
	if c.triggered >= len(c.steps) {
		c.Status = StatusCompleted
	}
	return step, num, true
}

// Resolve は入金等による解決で督促を止める。進行中のときだけ解決へ遷移する。
func (c *Campaign) Resolve() bool {
	if c.Status != StatusActive {
		return false
	}
	c.Status = StatusResolved
	return true
}

// BackfillAccount は請求先 ID が未設定の場合のみ補完する。
// 起票が請求先 ID の投影より先行した場合に、後から到達した投影で埋めるために使う。
func (c *Campaign) BackfillAccount(account shared.BillingAccountID) bool {
	if c.Account != "" || account == "" {
		return false
	}
	c.Account = account
	return true
}

// Triggered は実施済みステップ数。
func (c *Campaign) Triggered() int { return c.triggered }

// Total はシーケンスの全ステップ数。
func (c *Campaign) Total() int { return len(c.steps) }

// NextChannel は次に発火するチャネルを返す。
// 進行中(active)でない（入金解決・全実施済み）場合や全ステップ実施済みの場合は ""。
// 解決済みキャンペーンが「まだ次の督促がある」ように見えるのを防ぐ。
func (c *Campaign) NextChannel() Channel {
	if c.Status != StatusActive || c.triggered >= len(c.steps) {
		return ""
	}
	return c.steps[c.triggered].Channel
}
