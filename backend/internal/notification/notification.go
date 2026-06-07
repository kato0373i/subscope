// Package notification は通知モジュールの公開 API。
// dunning の DunningStepTriggered を購読し、チャネル（email/SMS/郵送）ごとの
// 送信を実体化する。送信手段の差異は Sender ACL に閉じ込め、既定はモック送信。
package notification

import (
	"context"
	"fmt"
	"log"

	"github.com/kato0373i/subscope/backend/internal/notification/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// Notification は通知レコードの再エクスポート。
type Notification = domain.Notification

// Sender は送信手段の ACL。メール/SMS/郵送ベンダ差異をこの境界に閉じ込める。
type Sender interface {
	Send(ctx context.Context, n *Notification) error
}

// MockSender は実際の送信を行わず成功扱いにする PoC 用スタブ。
type MockSender struct{}

func (MockSender) Send(_ context.Context, n *Notification) error {
	log.Printf("[notification] (mock) 送信 channel=%s to=%s invoice=%s", n.Channel, n.Recipient, n.Invoice)
	return nil
}

// Service は督促ステップを受けて通知を送信する。
type Service struct {
	bus    shared.EventBus
	sender Sender
	seq    int
}

// NewService はモック送信の Service を生成する。
func NewService(bus shared.EventBus) *Service {
	return NewServiceWithSender(bus, MockSender{})
}

// NewServiceWithSender は送信手段を注入できる Service を生成する。
// sender が nil の場合はモック送信にフォールバックする。
func NewServiceWithSender(bus shared.EventBus, sender Sender) *Service {
	if sender == nil {
		sender = MockSender{}
	}
	s := &Service{bus: bus, sender: sender}
	bus.Subscribe(events.NameDunningStepTriggered, s.onDunningStepTriggered)
	return s
}

func (s *Service) onDunningStepTriggered(ctx context.Context, e shared.Event) error {
	ev := e.(events.DunningStepTriggered)
	s.seq++
	n := domain.New(
		shared.NotificationID(fmt.Sprintf("NTF-%04d", s.seq)),
		ev.InvoiceID, ev.Account, ev.Channel, recipientFor(ev.Account, ev.Channel),
	)
	if err := s.sender.Send(ctx, n); err != nil {
		n.MarkFailed()
		log.Printf("[notification] 送信失敗 id=%s channel=%s err=%v", n.ID, n.Channel, err)
		return err
	}
	n.MarkSent()
	return s.bus.Publish(ctx, events.NotificationSent{
		NotificationID: n.ID,
		InvoiceID:      n.Invoice,
		Account:        n.Account,
		Channel:        n.Channel,
	})
}

// recipientFor は送信先を組み立てる。実運用では member/billing_account の連絡先を引く想定で、
// 現状は請求先 ID とチャネルから決まる暫定の宛先プレースホルダを返す。
func recipientFor(account shared.BillingAccountID, channel string) string {
	return fmt.Sprintf("%s:%s", channel, account)
}
