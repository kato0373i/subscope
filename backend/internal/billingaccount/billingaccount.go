// Package billingaccount は請求先管理モジュールの公開 API。
package billingaccount

import (
	"errors"
	"sync"

	"github.com/kato0373i/subscope/backend/internal/billingaccount/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
)

type Service struct {
	mu       sync.RWMutex
	accounts map[shared.BillingAccountID]*domain.BillingAccount
}

func NewService() *Service {
	return &Service{accounts: make(map[shared.BillingAccountID]*domain.BillingAccount)}
}

func (s *Service) Register(id shared.BillingAccountID, orgID shared.OrgID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.accounts[id]; exists {
		return errors.New("請求先 ID が重複しています")
	}
	a, err := domain.New(id, orgID, name)
	if err != nil {
		return err
	}
	s.accounts[id] = a
	return nil
}

func (s *Service) AddMember(accountID shared.BillingAccountID, memberID shared.MemberID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[accountID]
	if !ok {
		return errors.New("請求先が見つかりません")
	}
	a.AddMember(memberID)
	return nil
}

func (s *Service) Exists(id shared.BillingAccountID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.accounts[id]
	return ok
}
