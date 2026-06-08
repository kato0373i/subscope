// Package domain は webhook モジュールの集約・状態機械を閉じ込める private ドメイン層。
// 依存は shared と標準ライブラリのみ。
package domain

import "github.com/kato0373i/subscope/backend/internal/shared"

// Endpoint は配信先（会計ソフト連携・Slack 通知など）。
// 購読するイベント名の集合を持ち、該当イベントだけを配信対象にする。
type Endpoint struct {
	ID     shared.WebhookEndpointID
	URL    string
	events map[string]struct{}
	Active bool
}

// NewEndpoint は配信先を有効状態で生成する。eventNames は購読する統合イベント名。
func NewEndpoint(id shared.WebhookEndpointID, url string, eventNames []string) *Endpoint {
	set := make(map[string]struct{}, len(eventNames))
	for _, n := range eventNames {
		set[n] = struct{}{}
	}
	return &Endpoint{ID: id, URL: url, events: set, Active: true}
}

// Subscribes は指定イベントを購読しており、かつ有効なら true を返す。
func (e *Endpoint) Subscribes(eventName string) bool {
	if !e.Active {
		return false
	}
	_, ok := e.events[eventName]
	return ok
}

// Deactivate は配信先を無効化する（以降は配信対象から外れる）。
func (e *Endpoint) Deactivate() { e.Active = false }
