// Package organization はテナント管理モジュールの公開 API。
package organization

import (
	"errors"
	"sync"

	"github.com/kato0373i/subscope/backend/internal/organization/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
)

type Service struct {
	mu   sync.RWMutex
	orgs map[shared.OrgID]*domain.Organization
}

func NewService() *Service {
	return &Service{orgs: make(map[shared.OrgID]*domain.Organization)}
}

func (s *Service) Register(id shared.OrgID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.orgs[id]; exists {
		return errors.New("組織 ID が重複しています")
	}
	org, err := domain.New(id, name)
	if err != nil {
		return err
	}
	s.orgs[id] = org
	return nil
}

func (s *Service) Exists(id shared.OrgID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.orgs[id]
	return ok
}
