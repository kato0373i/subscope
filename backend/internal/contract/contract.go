// Package contract は契約モジュールの公開 API。
// 他モジュールはこのパッケージ（とイベント）だけに依存し、internal/domain には触れない。
package contract

import (
	"context"
	"errors"
	"log"
	"sort"
	"time"

	"github.com/kato0373i/subscope/backend/internal/contract/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// Adjustment は日割り調整明細の再エクスポート。
type Adjustment = domain.Adjustment

// ContractView は契約の読み取り専用ビュー（外向き読みモデル）。
// HTTP/読み取り層へ domain 集約を露出させないための DTO。
type ContractView struct {
	ID               shared.ContractID
	MemberID         shared.MemberID
	BillingAccountID shared.BillingAccountID
	MonthlyFee       shared.Money
	Status           string
}

// ErrNotFound は契約が見つからない場合に返る。
var ErrNotFound = errors.New("契約が見つかりません")

type Service struct {
	bus       shared.EventBus
	contracts map[shared.ContractID]*domain.Contract
}

func NewService(bus shared.EventBus) *Service {
	return &Service{bus: bus, contracts: make(map[shared.ContractID]*domain.Contract)}
}

// List は登録済み契約を ID 昇順で返す（読み取り API 用）。
func (s *Service) List() []ContractView {
	views := make([]ContractView, 0, len(s.contracts))
	for _, c := range s.contracts {
		views = append(views, ContractView{
			ID:               c.ID,
			MemberID:         c.MemberID,
			BillingAccountID: c.BillingAccountID,
			MonthlyFee:       c.MonthlyFee,
			Status:           string(c.Status),
		})
	}
	sort.Slice(views, func(i, j int) bool { return views[i].ID < views[j].ID })
	return views
}

// RegisterContract は契約を登録する（デモ用の簡易入口）。
func (s *Service) RegisterContract(id shared.ContractID, member shared.MemberID, account shared.BillingAccountID, fee shared.Money) {
	s.contracts[id] = domain.New(id, member, account, fee)
}

// RegisterTrial はトライアル付きで契約を登録する（trialing で開始）。
func (s *Service) RegisterTrial(id shared.ContractID, member shared.MemberID, account shared.BillingAccountID, fee shared.Money, trialDays int) {
	s.contracts[id] = domain.NewFull(id, "", member, account, "", fee, domain.CycleMonthly, domain.BillingAnchor(1), domain.TrialPeriod{Days: trialDays})
}

// MarkPastDue は未収発生により active → past_due へ遷移させる（請求オペレーションの入口）。
func (s *Service) MarkPastDue(id shared.ContractID) error {
	c, ok := s.contracts[id]
	if !ok {
		return ErrNotFound
	}
	return c.SetPastDue()
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

// Activate はトライアル終了で契約を有効化し ContractActivated を発行する。
func (s *Service) Activate(ctx context.Context, id shared.ContractID) error {
	c, ok := s.contracts[id]
	if !ok {
		return ErrNotFound
	}
	if err := c.Activate(); err != nil {
		return err
	}
	log.Printf("[contract] 契約を有効化 contract=%s", c.ID)
	return s.bus.Publish(ctx, events.ContractActivated{ContractID: c.ID})
}

// Suspend は契約を利用停止にし ContractSuspended を発行する。
func (s *Service) Suspend(ctx context.Context, id shared.ContractID) error {
	c, ok := s.contracts[id]
	if !ok {
		return ErrNotFound
	}
	if err := c.Suspend(); err != nil {
		return err
	}
	log.Printf("[contract] 契約を利用停止 contract=%s", c.ID)
	return s.bus.Publish(ctx, events.ContractSuspended{ContractID: c.ID})
}

// Cancel は契約を解約し ContractCancelled を発行する。
func (s *Service) Cancel(ctx context.Context, id shared.ContractID) error {
	c, ok := s.contracts[id]
	if !ok {
		return ErrNotFound
	}
	if err := c.Cancel(); err != nil {
		return err
	}
	log.Printf("[contract] 契約を解約 contract=%s", c.ID)
	return s.bus.Publish(ctx, events.ContractCancelled{ContractID: c.ID})
}

// ChangePlan はプランを変更する。変更日の日割り調整を計算し、契約を更新して
// PlanChanged（差額付き）を発行し、計算した調整明細を返す。
func (s *Service) ChangePlan(ctx context.Context, id shared.ContractID, newPlanID shared.PlanID, newFee shared.Money, changeDate time.Time) (Adjustment, error) {
	c, ok := s.contracts[id]
	if !ok {
		return Adjustment{}, ErrNotFound
	}
	adj, err := domain.ProrationPolicy{}.Calculate(c, newFee, changeDate)
	if err != nil {
		return Adjustment{}, err
	}
	oldPlan := c.PlanID
	if err := c.ChangePlan(newPlanID, newFee); err != nil {
		return Adjustment{}, err
	}
	log.Printf("[contract] プラン変更 contract=%s %s→%s 日割り差額=%s", c.ID, oldPlan, newPlanID, adj.Net)
	if err := s.bus.Publish(ctx, events.PlanChanged{
		ContractID:    c.ID,
		OldPlanID:     oldPlan,
		NewPlanID:     newPlanID,
		NetAdjustment: adj.Net,
	}); err != nil {
		return Adjustment{}, err
	}
	return adj, nil
}
