// Package contract は契約モジュールの公開 API。
// 他モジュールはこのパッケージ（とイベント）だけに依存し、internal/domain には触れない。
package contract

import (
	"context"
	"log"

	"github.com/kato0373i/subscope/backend/internal/contract/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

type Service struct {
	bus       shared.EventBus
	contracts map[shared.ContractID]*domain.Contract
}

func NewService(bus shared.EventBus) *Service {
	return &Service{bus: bus, contracts: make(map[shared.ContractID]*domain.Contract)}
}

// RegisterContract は契約を登録する（デモ用の簡易入口）。
func (s *Service) RegisterContract(id shared.ContractID, member shared.MemberID, account shared.BillingAccountID, fee shared.Money) {
	s.contracts[id] = domain.New(id, member, account, fee)
}

// TriggerBilling は請求サイクル到来を模した入口。BillingDue を発行する。
func (s *Service) TriggerBilling(ctx context.Context, id shared.ContractID) error {
	c, ok := s.contracts[id]
	if !ok {
		return nil
	}
	log.Printf("[contract]   請求サイクル到来 contract=%s account=%s", c.ID, c.BillingAccountID)
	return s.bus.Publish(ctx, events.BillingDue{
		ContractID:       c.ID,
		BillingAccountID: c.BillingAccountID,
		Amount:           c.MonthlyFee,
		Period:           "2026-06",
	})
}
