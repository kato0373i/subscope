package domain

import "github.com/kato0373i/subscope/backend/internal/shared"

type Status string

const (
	StatusPending   Status = "pending"   // 配信待ち（リトライ余地あり）
	StatusDelivered Status = "delivered" // 配信成功
	StatusFailed    Status = "failed"    // リトライ上限に達して失敗確定
)

// Delivery は 1 配信先への 1 イベント配信の試行記録。
// pending → delivered/failed の状態機械で、失敗時は上限までリトライする。
type Delivery struct {
	ID          shared.WebhookDeliveryID
	Endpoint    shared.WebhookEndpointID
	EventName   string
	Status      Status
	Attempts    int
	maxAttempts int
}

// NewDelivery は配信待ち（pending）の配信記録を生成する。
// maxAttempts が 1 未満の場合は 1 に補正する。
func NewDelivery(id shared.WebhookDeliveryID, endpoint shared.WebhookEndpointID, eventName string, maxAttempts int) *Delivery {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	return &Delivery{
		ID:          id,
		Endpoint:    endpoint,
		EventName:   eventName,
		Status:      StatusPending,
		maxAttempts: maxAttempts,
	}
}

// RecordSuccess は 1 回の配信成功を記録し delivered へ遷移する。pending からのみ。
func (d *Delivery) RecordSuccess() bool {
	if d.Status != StatusPending {
		return false
	}
	d.Attempts++
	d.Status = StatusDelivered
	return true
}

// RecordFailure は 1 回の配信失敗を記録する。pending からのみ。
// 試行回数が上限に達したら failed へ確定し、まだ余地があれば pending のまま。
func (d *Delivery) RecordFailure() bool {
	if d.Status != StatusPending {
		return false
	}
	d.Attempts++
	if d.Attempts >= d.maxAttempts {
		d.Status = StatusFailed
	}
	return true
}

// CanRetry は再試行できる（pending かつ上限未満）かを返す。
func (d *Delivery) CanRetry() bool {
	return d.Status == StatusPending && d.Attempts < d.maxAttempts
}
