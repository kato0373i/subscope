package shared

import "context"

// Event はモジュール間で受け渡される統合イベント。
// モジュール同士はこのイベント（＋型付き ID）でのみ会話する。
type Event interface {
	EventName() string
}

// Handler は単一の統合イベントを処理する。
type Handler func(ctx context.Context, e Event) error

// EventBus はモジュール間の非同期会話の唯一の経路。
// 具象実装（インメモリ / Outbox+メッセージング）は platform 層に置く。
type EventBus interface {
	Publish(ctx context.Context, events ...Event) error
	Subscribe(eventName string, h Handler)
}
