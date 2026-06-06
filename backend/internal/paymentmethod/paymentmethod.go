// Package paymentmethod は決済手段管理モジュールの公開 API。
// Invoice（債権）とは完全に独立しており、BillingAccount に属する。
package paymentmethod

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/kato0373i/subscope/backend/internal/paymentmethod/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

type Service struct {
	mu      sync.RWMutex
	bus     shared.EventBus
	methods map[shared.PaymentMethodID]*domain.PaymentMethod
}

func NewService(bus shared.EventBus) *Service {
	return &Service{bus: bus, methods: make(map[shared.PaymentMethodID]*domain.PaymentMethod)}
}

func (s *Service) RegisterCreditCard(ctx context.Context, id shared.PaymentMethodID, accountID shared.BillingAccountID, pspToken string, priority int) error {
	pm := domain.NewCreditCard(id, accountID, pspToken, priority)
	s.mu.Lock()
	s.methods[id] = pm
	s.mu.Unlock()
	log.Printf("[paymentmethod] クレジットカード登録 id=%s account=%s", id, accountID)
	if err := s.bus.Publish(ctx, events.PaymentMethodRegistered{
		PaymentMethodID:  id,
		BillingAccountID: accountID,
		MethodType:       string(domain.TypeCreditCard),
	}); err != nil {
		s.mu.Lock()
		delete(s.methods, id)
		s.mu.Unlock()
		return err
	}
	return nil
}

func (s *Service) RegisterBankAccount(ctx context.Context, id shared.PaymentMethodID, accountID shared.BillingAccountID, pspToken string, priority int) error {
	pm := domain.NewBankAccount(id, accountID, pspToken, priority)
	s.mu.Lock()
	s.methods[id] = pm
	s.mu.Unlock()
	log.Printf("[paymentmethod] 口座振替登録依頼 id=%s account=%s（審査待ち）", id, accountID)
	if err := s.bus.Publish(ctx, events.PaymentMethodRegistered{
		PaymentMethodID:  id,
		BillingAccountID: accountID,
		MethodType:       string(domain.TypeBankAccount),
	}); err != nil {
		s.mu.Lock()
		delete(s.methods, id)
		s.mu.Unlock()
		return err
	}
	return nil
}

func (s *Service) RegisterPaymentSlip(ctx context.Context, id shared.PaymentMethodID, accountID shared.BillingAccountID, priority int) error {
	pm := domain.NewPaymentSlip(id, accountID, priority)
	s.mu.Lock()
	s.methods[id] = pm
	s.mu.Unlock()
	log.Printf("[paymentmethod] 払込票登録 id=%s account=%s", id, accountID)
	if err := s.bus.Publish(ctx, events.PaymentMethodRegistered{
		PaymentMethodID:  id,
		BillingAccountID: accountID,
		MethodType:       string(domain.TypePaymentSlip),
	}); err != nil {
		s.mu.Lock()
		delete(s.methods, id)
		s.mu.Unlock()
		return err
	}
	return nil
}

// StartBankAccountReview は銀行審査開始を記録する（pending → reviewing）。
func (s *Service) StartBankAccountReview(_ context.Context, id shared.PaymentMethodID) error {
	s.mu.RLock()
	pm, ok := s.methods[id]
	s.mu.RUnlock()
	if !ok {
		return errors.New("決済手段が見つかりません")
	}
	return pm.StartReview()
}

// CompleteBankAccountRegistration は銀行審査の通過を記録する。
func (s *Service) CompleteBankAccountRegistration(ctx context.Context, id shared.PaymentMethodID) error {
	s.mu.RLock()
	pm, ok := s.methods[id]
	s.mu.RUnlock()
	if !ok {
		return errors.New("決済手段が見つかりません")
	}
	if err := pm.CompleteRegistration(); err != nil {
		return err
	}
	log.Printf("[paymentmethod] 口座振替 登録完了 id=%s（使用可能になりました）", id)
	return s.bus.Publish(ctx, events.BankAccountRegistrationCompleted{
		PaymentMethodID:  id,
		BillingAccountID: pm.BillingAccountID,
	})
}

// ExpireMethod はカード期限切れ等を記録する。
func (s *Service) ExpireMethod(ctx context.Context, id shared.PaymentMethodID) error {
	s.mu.RLock()
	pm, ok := s.methods[id]
	s.mu.RUnlock()
	if !ok {
		return errors.New("決済手段が見つかりません")
	}
	pm.Expire()
	log.Printf("[paymentmethod] 決済手段 失効 id=%s", id)
	return s.bus.Publish(ctx, events.PaymentMethodExpired{
		PaymentMethodID:  id,
		BillingAccountID: pm.BillingAccountID,
	})
}

// IsUsable は指定の手段が決済に使えるかを確認する。
func (s *Service) IsUsable(id shared.PaymentMethodID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pm, ok := s.methods[id]
	if !ok {
		return false
	}
	return pm.IsUsable()
}
