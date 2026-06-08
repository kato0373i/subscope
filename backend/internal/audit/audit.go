// Package audit は監査ログモジュールの公開 API。
// すべての統合イベントを購読し、追記専用・不変の監査ログとして記録する。
// 金融性が高いため「いつ・どのイベントが・どんな内容で」流れたかを残すのが責務で、
// 自身は新たなイベントを発行しない（純粋な記録係）。
package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/kato0373i/subscope/backend/internal/audit/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// Entry は監査ログ行の再エクスポート。
type Entry = domain.Entry

// Clock は記録時刻の供給源。テストで固定時刻を注入するための ACL。
type Clock func() time.Time

// Service は全統合イベントを購読し、追記専用の監査ログに記録する。
// 状態は同期インメモリバス前提で非ガード（コードベース共通の方針）。
type Service struct {
	clock   Clock
	seq     int
	entries []Entry
}

// NewService は実時刻で記録する Service を生成し、全統合イベントを購読する。
func NewService(bus shared.EventBus) *Service {
	return NewServiceWithClock(bus, time.Now)
}

// NewServiceWithClock は記録時刻を注入できる Service を生成する。
// clock が nil の場合は実時刻にフォールバックする。
func NewServiceWithClock(bus shared.EventBus, clock Clock) *Service {
	if clock == nil {
		clock = time.Now
	}
	s := &Service{clock: clock}
	for _, name := range events.AllNames() {
		bus.Subscribe(name, s.record)
	}
	return s
}

func (s *Service) record(_ context.Context, e shared.Event) error {
	s.seq++
	id := shared.AuditLogID(fmt.Sprintf("AUD-%06d", s.seq))
	entry := domain.NewEntry(id, e.EventName(), fmt.Sprintf("%+v", e), s.clock())
	s.entries = append(s.entries, entry)
	return nil
}

// Entries は記録済みの監査ログをコピーして返す（読み取り専用の投影）。
func (s *Service) Entries() []Entry {
	return append([]Entry(nil), s.entries...)
}

// Len は記録済みの監査ログ件数を返す。
func (s *Service) Len() int {
	return len(s.entries)
}
