// Package member は会員管理モジュールの公開 API。
package member

import (
	"github.com/kato0373i/subscope/backend/internal/member/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
)

type Service struct {
	members map[shared.MemberID]*domain.Member
}

func NewService() *Service {
	return &Service{members: make(map[shared.MemberID]*domain.Member)}
}

func (s *Service) Register(id shared.MemberID, orgID shared.OrgID, name, email string) {
	s.members[id] = domain.New(id, orgID, name, email)
}

func (s *Service) Exists(id shared.MemberID) bool {
	_, ok := s.members[id]
	return ok
}
