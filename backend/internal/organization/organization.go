// Package organization はテナント管理モジュールの公開 API。
package organization

import (
	"github.com/kato0373i/subscope/backend/internal/organization/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
)

type Service struct {
	orgs map[shared.OrgID]*domain.Organization
}

func NewService() *Service {
	return &Service{orgs: make(map[shared.OrgID]*domain.Organization)}
}

func (s *Service) Register(id shared.OrgID, name string) {
	s.orgs[id] = domain.New(id, name)
}

func (s *Service) Exists(id shared.OrgID) bool {
	_, ok := s.orgs[id]
	return ok
}
