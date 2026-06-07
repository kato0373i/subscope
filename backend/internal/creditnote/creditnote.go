// Package creditnote は赤伝（適格返還請求書）モジュールの公開 API。
// プラン変更のダウングレード差額（PlanChanged の負の差額）など返金事由を受けて
// CreditNote を発行し、CreditNoteIssued を発行する。手動発行（解約返金等）も提供する。
package creditnote

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"

	"github.com/kato0373i/subscope/backend/internal/creditnote/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// ErrNotFound は赤伝が見つからない場合に返る。
var ErrNotFound = errors.New("赤伝が見つかりません")

// ErrAmountOverflow は返金額の符号反転で int64 がオーバーフローする場合に返る。
var ErrAmountOverflow = errors.New("返金額が int64 の範囲を超えています")

type Service struct {
	bus   shared.EventBus
	notes map[shared.CreditNoteID]*domain.CreditNote
	seq   int
}

func NewService(bus shared.EventBus) *Service {
	s := &Service{bus: bus, notes: make(map[shared.CreditNoteID]*domain.CreditNote)}
	// プラン変更のダウングレード（差額が負）を返金として赤伝化する。
	bus.Subscribe(events.NamePlanChanged, s.onPlanChanged)
	return s
}

func (s *Service) onPlanChanged(ctx context.Context, e shared.Event) error {
	ev := e.(events.PlanChanged)
	// 差額が負（返金）の場合のみ赤伝を発行する。追加請求（正）は対象外。
	if !ev.NetAdjustment.IsNegative() {
		return nil
	}
	// 符号反転は math.MinInt64 でオーバーフローする（円・int64 のガイドライン）。
	if ev.NetAdjustment.Amount == math.MinInt64 {
		return ErrAmountOverflow
	}
	refund := shared.Money{Amount: -ev.NetAdjustment.Amount, Currency: ev.NetAdjustment.Currency}
	_, err := s.issue(ctx, ev.ContractID, refund, "plan_downgrade")
	return err
}

// Issue は返金事由（解約返金など）に対して赤伝を手動発行する。
func (s *Service) Issue(ctx context.Context, contract shared.ContractID, amount shared.Money, reason string) (shared.CreditNoteID, error) {
	return s.issue(ctx, contract, amount, reason)
}

func (s *Service) issue(ctx context.Context, contract shared.ContractID, amount shared.Money, reason string) (shared.CreditNoteID, error) {
	s.seq++
	id := shared.CreditNoteID(fmt.Sprintf("CN-%04d", s.seq))
	n, err := domain.New(id, contract, amount, reason)
	if err != nil {
		return "", err
	}
	s.notes[id] = n
	log.Printf("[creditnote] 赤伝を発行 id=%s contract=%s amount=%s reason=%s", n.ID, contract, amount, reason)
	if err := s.bus.Publish(ctx, events.CreditNoteIssued{
		CreditNoteID: n.ID,
		ContractID:   contract,
		Amount:       amount,
		Reason:       reason,
	}); err != nil {
		return "", err
	}
	return id, nil
}

// Apply は赤伝の返金適用を記録する。
func (s *Service) Apply(id shared.CreditNoteID) error {
	n, ok := s.notes[id]
	if !ok {
		return ErrNotFound
	}
	return n.Apply()
}
