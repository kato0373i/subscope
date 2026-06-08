// Package domain は audit モジュールの集約を閉じ込める private ドメイン層。
// 依存は shared と標準ライブラリのみ。
package domain

import (
	"time"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

// Entry は監査ログの 1 行。金融性が高いため全コマンド（統合イベント）を記録する。
// 追記専用・不変であり、生成後にフィールドを変更するメソッドは持たない。
type Entry struct {
	id         shared.AuditLogID
	eventName  string
	detail     string
	recordedAt time.Time
}

// NewEntry は不変の監査ログ行を生成する。
func NewEntry(id shared.AuditLogID, eventName, detail string, recordedAt time.Time) Entry {
	return Entry{
		id:         id,
		eventName:  eventName,
		detail:     detail,
		recordedAt: recordedAt,
	}
}

// ID は監査ログ ID を返す。
func (e Entry) ID() shared.AuditLogID { return e.id }

// EventName は記録対象の統合イベント名を返す。
func (e Entry) EventName() string { return e.eventName }

// Detail はイベント内容のスナップショット（文字列表現）を返す。
func (e Entry) Detail() string { return e.detail }

// RecordedAt は記録時刻を返す。
func (e Entry) RecordedAt() time.Time { return e.recordedAt }
