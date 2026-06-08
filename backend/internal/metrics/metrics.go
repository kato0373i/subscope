// Package metrics は指標モジュールの公開 API。
// 中核フローの統合イベントを購読し、MRR/解約/回収などの指標を投影する CQRS の Read Model。
// 書き込み集約ではなく、各イベントを集計して「現在値」を答えるのが責務（イベントは発行しない）。
package metrics

import (
	"context"

	"github.com/kato0373i/subscope/backend/internal/metrics/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// Snapshot は投影された指標の読み取り専用スナップショット。
type Snapshot = domain.Projection

// Service はイベントを集計して指標を投影する。
// 状態は同期インメモリバス前提で非ガード（コードベース共通の方針）。
type Service struct {
	projection domain.Projection
}

// NewService は指標サービスを生成し、集計対象のイベントを購読する。
func NewService(bus shared.EventBus) *Service {
	s := &Service{}
	bus.Subscribe(events.NameContractActivated, s.onContractActivated)
	bus.Subscribe(events.NameContractCancelled, s.onContractCancelled)
	bus.Subscribe(events.NameInvoiceIssued, s.onInvoiceIssued)
	bus.Subscribe(events.NameInvoicePaid, s.onInvoicePaid)
	bus.Subscribe(events.NameCollectionRecovered, s.onCollectionRecovered)
	bus.Subscribe(events.NameCollectionWrittenOff, s.onCollectionWrittenOff)
	bus.Subscribe(events.NameCreditNoteIssued, s.onCreditNoteIssued)
	return s
}

func (s *Service) onContractActivated(_ context.Context, e shared.Event) error {
	s.projection.OnContractActivated(e.(events.ContractActivated).ContractID)
	return nil
}

func (s *Service) onContractCancelled(_ context.Context, e shared.Event) error {
	s.projection.OnContractCancelled(e.(events.ContractCancelled).ContractID)
	return nil
}

func (s *Service) onInvoiceIssued(_ context.Context, e shared.Event) error {
	return s.projection.OnInvoiceIssued(e.(events.InvoiceIssued).Amount)
}

func (s *Service) onInvoicePaid(_ context.Context, _ shared.Event) error {
	s.projection.OnInvoicePaid()
	return nil
}

func (s *Service) onCollectionRecovered(_ context.Context, e shared.Event) error {
	return s.projection.OnCollectionRecovered(e.(events.CollectionRecovered).Amount)
}

func (s *Service) onCollectionWrittenOff(_ context.Context, e shared.Event) error {
	return s.projection.OnCollectionWrittenOff(e.(events.CollectionWrittenOff).Amount)
}

func (s *Service) onCreditNoteIssued(_ context.Context, e shared.Event) error {
	return s.projection.OnCreditNoteIssued(e.(events.CreditNoteIssued).Amount)
}

// Snapshot は現在の指標投影をコピーして返す。
func (s *Service) Snapshot() Snapshot {
	return s.projection
}
