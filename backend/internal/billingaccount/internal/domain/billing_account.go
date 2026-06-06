package domain

import (
	"errors"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

// BillingAccount は請求先・支払者の集約。
// 複数の Member を束ねられる（団体一括請求）。
// 決済手段（PaymentMethod）はこの集約に属する。
type BillingAccount struct {
	ID        shared.BillingAccountID
	OrgID     shared.OrgID
	Name      string
	MemberIDs []shared.MemberID
}

func New(id shared.BillingAccountID, orgID shared.OrgID, name string) (*BillingAccount, error) {
	if name == "" {
		return nil, errors.New("請求先名は必須です")
	}
	return &BillingAccount{
		ID:        id,
		OrgID:     orgID,
		Name:      name,
		MemberIDs: []shared.MemberID{},
	}, nil
}

// AddMember は BillingAccount に会員を紐付ける（重複は無視）。
func (b *BillingAccount) AddMember(memberID shared.MemberID) {
	for _, id := range b.MemberIDs {
		if id == memberID {
			return
		}
	}
	b.MemberIDs = append(b.MemberIDs, memberID)
}
