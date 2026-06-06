// Package billingaccount は請求先管理モジュールの公開 API。
package billingaccount

import (
	"github.com/kato0373i/subscope/backend/internal/billingaccount/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
)

type Service struct {
	accounts map[shared.BillingAccountID]*domain.BillingAccount
}

func NewService() *Service {
	return &Service{accounts: make(map[shared.BillingAccountID]*domain.BillingAccount)}
}

func (s *Service) Register(id shared.BillingAccountID, orgID shared.OrgID, name string) {
	s.accounts[id] = domain.New(id, orgID, name)
}

func (s *Service) AddMember(accountID shared.BillingAccountID, memberID shared.MemberID) {
	if a, ok := s.accounts[accountID]; ok {
		a.AddMember(memberID)
	}
}

func (s *Service) Exists(id shared.BillingAccountID) bool {
	_, ok := s.accounts[id]
	return ok
}
