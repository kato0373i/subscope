package domain

import "github.com/kato0373i/subscope/backend/internal/shared"

type Status string

const (
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
)

// Organization はテナント境界。全集約に OrgID を持たせることでマルチテナントを実現する。
type Organization struct {
	ID     shared.OrgID
	Name   string
	Status Status
}

func New(id shared.OrgID, name string) *Organization {
	return &Organization{ID: id, Name: name, Status: StatusActive}
}

func (o *Organization) Suspend()  { o.Status = StatusSuspended }
func (o *Organization) Activate() { o.Status = StatusActive }
