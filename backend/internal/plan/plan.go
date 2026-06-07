// Package plan はプランマスタモジュールの公開 API。
// 課金プラン（Plan / Price / BillingInterval）を管理する。
// billing は請求確定時に Snapshot を取得し、発行済 Invoice へ金額を焼き込む想定。
package plan

import (
	"errors"
	"sync"

	"github.com/kato0373i/subscope/backend/internal/plan/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
)

// 内部ドメイン型を再エクスポートし、外部からは plan.* で参照させる。
type (
	Price           = domain.Price
	BillingInterval = domain.BillingInterval
)

const (
	IntervalMonthly = domain.IntervalMonthly
	IntervalYearly  = domain.IntervalYearly
)

// NewPrice は価格 VO を生成する。
func NewPrice(amount shared.Money, interval BillingInterval) (Price, error) {
	return domain.NewPrice(amount, interval)
}

var ErrDuplicatePlan = errors.New("プラン ID が重複しています")

// ErrNotFound はプランが見つからない場合に返る。
var ErrNotFound = errors.New("プランが見つかりません")

type Service struct {
	mu    sync.RWMutex
	plans map[shared.PlanID]*domain.Plan
}

func NewService() *Service {
	return &Service{plans: make(map[shared.PlanID]*domain.Plan)}
}

// Register は新しいプランを登録する。
func (s *Service) Register(id shared.PlanID, orgID shared.OrgID, name string, price Price) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.plans[id]; ok {
		return ErrDuplicatePlan
	}
	p, err := domain.New(id, orgID, name, price)
	if err != nil {
		return err
	}
	s.plans[id] = p
	return nil
}

// ChangePrice は登録済プランの価格を改定する。
func (s *Service) ChangePrice(id shared.PlanID, price Price) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.plans[id]
	if !ok {
		return ErrNotFound
	}
	p.ChangePrice(price)
	return nil
}

// Snapshot は Invoice へ焼き込むための現在価格スナップショットを返す。
func (s *Service) Snapshot(id shared.PlanID) (Price, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.plans[id]
	if !ok {
		return Price{}, ErrNotFound
	}
	return p.Snapshot(), nil
}
