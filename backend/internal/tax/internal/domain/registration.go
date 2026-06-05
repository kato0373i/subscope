package domain

import (
	"fmt"
	"regexp"
	"time"
)

// qualifiedNoPattern は適格請求書発行事業者の登録番号の書式（"T" + 13桁）。
var qualifiedNoPattern = regexp.MustCompile(`^T\d{13}$`)

// Registration は適格請求書発行事業者の登録。発行済み請求書には
// この登録番号をスナップショットとして焼き込む。
type Registration struct {
	Number     string
	ValidFrom  time.Time
	ValidUntil *time.Time // nil は無期限
}

// NewRegistration は登録番号の書式を検証して Registration を生成する。
func NewRegistration(number string, validFrom time.Time) (Registration, error) {
	if !qualifiedNoPattern.MatchString(number) {
		return Registration{}, fmt.Errorf("不正な登録番号です（T+13桁が必要）: %q", number)
	}
	return Registration{Number: number, ValidFrom: validFrom}, nil
}

// IsValidAt は指定時点で登録が有効かを返す。
func (r Registration) IsValidAt(t time.Time) bool {
	if t.Before(r.ValidFrom) {
		return false
	}
	if r.ValidUntil != nil && t.After(*r.ValidUntil) {
		return false
	}
	return true
}
