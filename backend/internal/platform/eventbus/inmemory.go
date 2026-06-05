// Package eventbus はインメモリのイベントバス実装を提供する（開発・テスト用）。
// 本番では Outbox パターン＋メッセージングに差し替える前提。
package eventbus

import (
	"context"
	"sync"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

// InMemory は同期ディスパッチのインメモリ EventBus。
type InMemory struct {
	mu       sync.RWMutex
	handlers map[string][]shared.Handler
}

func NewInMemory() *InMemory {
	return &InMemory{handlers: make(map[string][]shared.Handler)}
}

func (b *InMemory) Subscribe(name string, h shared.Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[name] = append(b.handlers[name], h)
}

func (b *InMemory) Publish(ctx context.Context, events ...shared.Event) error {
	for _, e := range events {
		b.mu.RLock()
		hs := append([]shared.Handler(nil), b.handlers[e.EventName()]...)
		b.mu.RUnlock()
		for _, h := range hs {
			if err := h(ctx, e); err != nil {
				return err
			}
		}
	}
	return nil
}
