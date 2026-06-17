// Package dunning は督促モジュールの公開 API。
// 回収が行き詰まった（決済失敗・エスカレーション）未収に対し、督促シーケンスを
// 段階的に進め、各ステップを DunningStepTriggered として発行する。
// 実際の送信は notification が担い、入金で督促は止まる。
package dunning

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/kato0373i/subscope/backend/internal/dunning/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// 督促シーケンスの型を公開パッケージから再エクスポートする。
type (
	Step    = domain.Step
	Channel = domain.Channel
)

// DefaultSequence は既定の督促シーケンス（D+0 メール → D+3 SMS → D+7 督促状）。
func DefaultSequence() []Step { return domain.DefaultSequence() }

// CampaignView は督促キャンペーンの外向き読み取りビュー。
type CampaignView struct {
	CampaignID     shared.DunningCampaignID
	InvoiceID      shared.InvoiceID
	Account        shared.BillingAccountID
	Status         string // active / resolved / completed
	StepsTriggered int
	StepsTotal     int
	NextChannel    string // 次段階のチャネル（完了なら ""）
}

// ListCampaigns は進行中・終了済みを含む全督促キャンペーンを返す。
// map 反復は順不同のため、呼び出し側で安定ソートする想定（CampaignID 昇順）。
// HTTP の読み取りとイベント処理（別ゴルーチン）の競合を避けるため RLock で保護する。
func (s *Service) ListCampaigns() []CampaignView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]CampaignView, 0, len(s.campaigns))
	for _, c := range s.campaigns {
		out = append(out, CampaignView{
			CampaignID:     c.ID,
			InvoiceID:      c.Invoice,
			Account:        c.Account,
			Status:         string(c.Status),
			StepsTriggered: c.Triggered(),
			StepsTotal:     c.Total(),
			NextChannel:    string(c.NextChannel()),
		})
	}
	return out
}

// Service は督促キャンペーンを統括する。1 請求につき 1 キャンペーン。
type Service struct {
	// mu は campaigns/accounts と seq を保護する。HTTP の読み取り（ListCampaigns）と
	// イベント処理（書き込み）が別ゴルーチンで競合しても concurrent map access で
	// panic しないようにする。同期バスはハンドラを発行ゴルーチン上で逐次実行し、
	// dunning ハンドラから dunning 自身へ再入する経路は無いため粗粒度ロックで安全。
	mu        sync.RWMutex
	bus       shared.EventBus
	steps     []domain.Step
	campaigns map[shared.InvoiceID]*domain.Campaign
	accounts  map[shared.InvoiceID]shared.BillingAccountID
	seq       int
}

// NewService は既定シーケンスで Service を生成する。
func NewService(bus shared.EventBus) *Service {
	return NewServiceWithSequence(bus, nil)
}

// NewServiceWithSequence は督促シーケンスを差し替えられる Service を生成する。
// steps が nil の場合は既定シーケンスを使う。
func NewServiceWithSequence(bus shared.EventBus, steps []Step) *Service {
	s := &Service{
		bus:       bus,
		steps:     steps,
		campaigns: make(map[shared.InvoiceID]*domain.Campaign),
		accounts:  make(map[shared.InvoiceID]shared.BillingAccountID),
	}
	// 未収の請求先を知るために発行を購読し、請求先 ID を投影する。
	bus.Subscribe(events.NameInvoiceIssued, s.onInvoiceIssued)
	// 決済失敗・エスカレーションで督促を起票する。
	bus.Subscribe(events.NamePaymentFailed, s.onPaymentFailed)
	bus.Subscribe(events.NameCollectionEscalated, s.onCollectionEscalated)
	// 入金で督促を止める。
	bus.Subscribe(events.NameInvoicePaid, s.onInvoicePaid)
	return s
}

func (s *Service) onInvoiceIssued(_ context.Context, e shared.Event) error {
	ev := e.(events.InvoiceIssued)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[ev.InvoiceID] = ev.BillingAccountID
	// 起票が投影に先行していた場合、後から到達した請求先 ID で既存キャンペーンを補完する。
	if c, ok := s.campaigns[ev.InvoiceID]; ok {
		c.BackfillAccount(ev.BillingAccountID)
	}
	return nil
}

func (s *Service) onPaymentFailed(ctx context.Context, e shared.Event) error {
	ev := e.(events.PaymentFailed)
	return s.startCampaign(ctx, ev.InvoiceID)
}

func (s *Service) onCollectionEscalated(ctx context.Context, e shared.Event) error {
	ev := e.(events.CollectionEscalated)
	return s.startCampaign(ctx, ev.InvoiceID)
}

func (s *Service) onInvoicePaid(_ context.Context, e shared.Event) error {
	ev := e.(events.InvoicePaid)
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.campaigns[ev.InvoiceID]; ok && c.Resolve() {
		log.Printf("[dunning] 入金により督促を停止 campaign=%s invoice=%s", c.ID, ev.InvoiceID)
	}
	return nil
}

// startCampaign は請求に対する督促を起票し、最初のステップを発火する。
// 既にキャンペーンがあれば何もしない（重複した失敗通知で多重起票しない）。
func (s *Service) startCampaign(ctx context.Context, invoice shared.InvoiceID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.campaigns[invoice]; ok {
		return nil
	}
	account, ok := s.accounts[invoice]
	if !ok {
		// InvoiceIssued の投影が未到達でも督促は止めず起票する。
		// 後から投影が到達した時点で onInvoiceIssued が BackfillAccount で補完する。
		log.Printf("[dunning] 警告: 請求先 ID 未投影のまま起票 invoice=%s", invoice)
	}
	s.seq++
	c := domain.NewCampaign(
		shared.DunningCampaignID(fmt.Sprintf("DUN-%04d", s.seq)),
		invoice, account, s.steps,
	)
	s.campaigns[invoice] = c
	log.Printf("[dunning] 督促を起票 campaign=%s invoice=%s", c.ID, invoice)
	return s.triggerNext(ctx, c)
}

// AdvanceCampaigns は進行中の全キャンペーンの次ステップを 1 つ発火する。
// 時間経過で次の督促段階に進めるスケジューラのティックを表す。
// 1 件のエラーで中断せず全件処理し、エラーは集約して返す。
func (s *Service) AdvanceCampaigns(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var errs []error
	for _, c := range s.campaigns {
		if err := s.triggerNext(ctx, c); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// triggerNext はキャンペーンの次ステップを発火し DunningStepTriggered を発行する。
func (s *Service) triggerNext(ctx context.Context, c *domain.Campaign) error {
	step, num, ok := c.TriggerNext()
	if !ok {
		return nil
	}
	log.Printf("[dunning] 督促ステップ発火 campaign=%s step=%d channel=%s", c.ID, num, step.Channel)
	return s.bus.Publish(ctx, events.DunningStepTriggered{
		CampaignID: c.ID,
		InvoiceID:  c.Invoice,
		Account:    c.Account,
		Channel:    string(step.Channel),
		StepNumber: num,
	})
}
