package domain

import "github.com/kato0373i/subscope/backend/internal/shared"

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
)

// Member はサービスを受ける主体。請求の宛先ではない点が設計の核心。
// 支払責任は BillingAccount が持つ。
type Member struct {
	ID     shared.MemberID
	OrgID  shared.OrgID
	Name   string
	Email  string
	Status Status
}

func New(id shared.MemberID, orgID shared.OrgID, name, email string) *Member {
	return &Member{
		ID:     id,
		OrgID:  orgID,
		Name:   name,
		Email:  email,
		Status: StatusActive,
	}
}

func (m *Member) Deactivate() { m.Status = StatusInactive }
func (m *Member) Activate()   { m.Status = StatusActive }
