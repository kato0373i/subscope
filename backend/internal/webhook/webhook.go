// Package webhook は外部連携モジュールの公開 API。
// 統合イベントを購読し、登録済みの配信先（会計ソフト連携・Slack 通知など）へ
// イベントを配信する。配信は Transport ACL に閉じ込め、失敗時は上限までリトライする。
// 自身は新たな統合イベントを発行しない。
package webhook

import (
	"context"
	"fmt"
	"log"

	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
	"github.com/kato0373i/subscope/backend/internal/webhook/internal/domain"
)

// Delivery は配信記録の再エクスポート。
type Delivery = domain.Delivery

// defaultMaxAttempts は 1 配信あたりのリトライ上限。
const defaultMaxAttempts = 3

// Transport は実際の配信手段の ACL。HTTP POST やベンダ SDK の差異をここに閉じ込める。
type Transport interface {
	Deliver(ctx context.Context, url, eventName, payload string) error
}

// MockTransport は実際の送信を行わず成功扱いにする PoC 用スタブ。
type MockTransport struct{}

func (MockTransport) Deliver(_ context.Context, url, eventName, _ string) error {
	log.Printf("[webhook] (mock) 配信 url=%s event=%s", url, eventName)
	return nil
}

// Service は配信先を管理し、購読イベントを各配信先へ届ける。
// 状態は同期インメモリバス前提で非ガード（コードベース共通の方針）。
type Service struct {
	transport   Transport
	maxAttempts int
	seq         int
	endpoints   map[shared.WebhookEndpointID]*domain.Endpoint
	deliveries  []*Delivery
}

// NewService はモック配信の Service を生成し、全統合イベントを購読する。
func NewService(bus shared.EventBus) *Service {
	return NewServiceWithTransport(bus, MockTransport{})
}

// NewServiceWithTransport は配信手段を注入できる Service を生成する。
// transport が nil の場合はモック配信にフォールバックする。
func NewServiceWithTransport(bus shared.EventBus, transport Transport) *Service {
	if transport == nil {
		transport = MockTransport{}
	}
	s := &Service{
		transport:   transport,
		maxAttempts: defaultMaxAttempts,
		endpoints:   make(map[shared.WebhookEndpointID]*domain.Endpoint),
	}
	for _, name := range events.AllNames() {
		bus.Subscribe(name, s.dispatch)
	}
	return s
}

// RegisterEndpoint は配信先を登録する。eventNames は購読する統合イベント名。
func (s *Service) RegisterEndpoint(id shared.WebhookEndpointID, url string, eventNames ...string) {
	s.endpoints[id] = domain.NewEndpoint(id, url, eventNames)
}

func (s *Service) dispatch(ctx context.Context, e shared.Event) error {
	payload := fmt.Sprintf("%+v", e)
	for _, ep := range s.endpoints {
		if !ep.Subscribes(e.EventName()) {
			continue
		}
		s.seq++
		d := domain.NewDelivery(
			shared.WebhookDeliveryID(fmt.Sprintf("WD-%06d", s.seq)),
			ep.ID, e.EventName(), s.maxAttempts,
		)
		s.attempt(ctx, ep, d, payload)
		s.deliveries = append(s.deliveries, d)
	}
	return nil
}

// attempt は配信を上限までリトライする。1 回でも成功すれば delivered で打ち切る。
// 配信失敗はバス全体を止めない（記録に残し、購読チェーンは継続する）。
func (s *Service) attempt(ctx context.Context, ep *domain.Endpoint, d *Delivery, payload string) {
	for d.CanRetry() {
		if err := s.transport.Deliver(ctx, ep.URL, d.EventName, payload); err != nil {
			d.RecordFailure()
			log.Printf("[webhook] 配信失敗 id=%s url=%s event=%s attempt=%d err=%v",
				d.ID, ep.URL, d.EventName, d.Attempts, err)
			continue
		}
		d.RecordSuccess()
		return
	}
}

// Deliveries は配信記録をコピーして返す（読み取り専用の投影）。
func (s *Service) Deliveries() []*Delivery {
	return append([]*Delivery(nil), s.deliveries...)
}
