// Package domain は通知モジュールの集約・状態機械を閉じ込める private ドメイン層。
// 依存は shared と標準ライブラリのみ。
package domain

import "github.com/kato0373i/subscope/backend/internal/shared"

type Status string

const (
	StatusQueued Status = "queued" // 送信前
	StatusSent   Status = "sent"   // 送信完了
	StatusFailed Status = "failed" // 送信失敗
)

// Notification は 1 件の通知。queued → sent/failed の状態機械で送信結果を保持する。
type Notification struct {
	ID        shared.NotificationID
	Invoice   shared.InvoiceID
	Account   shared.BillingAccountID
	Channel   string
	Recipient string
	Status    Status
}

// New は送信前（queued）の通知を生成する。
func New(id shared.NotificationID, invoice shared.InvoiceID, account shared.BillingAccountID, channel, recipient string) *Notification {
	return &Notification{
		ID:        id,
		Invoice:   invoice,
		Account:   account,
		Channel:   channel,
		Recipient: recipient,
		Status:    StatusQueued,
	}
}

// MarkSent は送信成功へ遷移する。queued からのみ。
func (n *Notification) MarkSent() bool {
	if n.Status != StatusQueued {
		return false
	}
	n.Status = StatusSent
	return true
}

// MarkFailed は送信失敗へ遷移する。queued からのみ。
func (n *Notification) MarkFailed() bool {
	if n.Status != StatusQueued {
		return false
	}
	n.Status = StatusFailed
	return true
}
