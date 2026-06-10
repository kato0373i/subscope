// Package member は会員管理モジュールの公開 API。
package member

import (
	"sync"

	"github.com/kato0373i/subscope/backend/internal/member/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
)

type Service struct {
	mu      sync.RWMutex
	members map[shared.MemberID]*domain.Member
}

func NewService() *Service {
	return &Service{members: make(map[shared.MemberID]*domain.Member)}
}

func (s *Service) Register(id shared.MemberID, orgID shared.OrgID, name, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := domain.New(id, orgID, name, email)
	if err != nil {
		return err
	}
	s.members[id] = m
	return nil
}

func (s *Service) Exists(id shared.MemberID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.members[id]
	return ok
}

// Name は会員の表示名を返す（読み取り API 用）。未登録なら ok=false。
func (s *Service) Name(id shared.MemberID) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.members[id]
	if !ok {
		return "", false
	}
	return m.Name, true
}
