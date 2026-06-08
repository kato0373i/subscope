// Package domain は creditnote モジュールの集約・状態機械を閉じ込める private ドメイン層。
// 依存は shared と標準ライブラリのみ。
package domain

import (
	"errors"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

var (
	// ErrNonPositiveAmount は 0 円以下の返金額を弾く。
	ErrNonPositiveAmount = errors.New("返金額は 1 円以上である必要があります")
	// ErrNotIssued は issued 以外からの状態遷移を弾く。
	ErrNotIssued = errors.New("発行済み(issued)でない赤伝は適用できません")
)

type Status string

const (
	StatusIssued  Status = "issued"  // 発行済み（返金確定前）
	StatusApplied Status = "applied" // 返金が実適用された
)

// CreditNote は赤伝（適格返還請求書）。契約に対する返金・取消を表す独立した集約。
// インボイス制度では返還は元請求書の修正ではなく別文書（適格返還請求書）で行うため、
// Invoice の負債行ではなく独立集約として表現する。
type CreditNote struct {
	ID       shared.CreditNoteID
	Contract shared.ContractID
	Amount   shared.Money // 返金額（正の値）
	Reason   string
	Status   Status
}

// New は赤伝を発行する（issued）。返金額は正でなければならない。
func New(id shared.CreditNoteID, contract shared.ContractID, amount shared.Money, reason string) (*CreditNote, error) {
	if amount.Amount <= 0 {
		return nil, ErrNonPositiveAmount
	}
	return &CreditNote{
		ID:       id,
		Contract: contract,
		Amount:   amount,
		Reason:   reason,
		Status:   StatusIssued,
	}, nil
}

// Apply は返金が実際に適用されたことを記録する。issued からのみ。
func (n *CreditNote) Apply() error {
	if n.Status != StatusIssued {
		return ErrNotIssued
	}
	n.Status = StatusApplied
	return nil
}
