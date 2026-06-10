// Package billing は請求モジュールの公開 API。Invoice（債権）を生成・管理する。
package billing

import (
	"context"
	"fmt"
	"log"
	"sort"

	"github.com/kato0373i/subscope/backend/internal/billing/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// InvoiceView は請求書の読み取り専用ビュー（外向き読みモデル）。
type InvoiceView struct {
	ID               shared.InvoiceID
	ContractID       shared.ContractID
	BillingAccountID shared.BillingAccountID
	Amount           shared.Money
	Status           string
}

type Service struct {
	bus      shared.EventBus
	invoices map[shared.InvoiceID]*domain.Invoice
	seq      int
}

func NewService(bus shared.EventBus) *Service {
	s := &Service{bus: bus, invoices: make(map[shared.InvoiceID]*domain.Invoice)}
	bus.Subscribe(events.NameBillingDue, s.onBillingDue)
	bus.Subscribe(events.NameInvoicePaid, s.onInvoicePaid)
	return s
}

// ListInvoices は発行済み請求書を ID 昇順で返す（読み取り API 用）。
func (s *Service) ListInvoices() []InvoiceView {
	views := make([]InvoiceView, 0, len(s.invoices))
	for _, inv := range s.invoices {
		views = append(views, InvoiceView{
			ID:               inv.ID,
			ContractID:       inv.ContractID,
			BillingAccountID: inv.BillingAccountID,
			Amount:           inv.Amount,
			Status:           string(inv.Status),
		})
	}
	sort.Slice(views, func(i, j int) bool { return views[i].ID < views[j].ID })
	return views
}

func (s *Service) onBillingDue(ctx context.Context, e shared.Event) error {
	ev := e.(events.BillingDue)
	s.seq++
	id := shared.InvoiceID(fmt.Sprintf("INV-%04d", s.seq))
	inv := domain.NewIssued(id, ev.ContractID, ev.BillingAccountID, ev.Amount)
	s.invoices[id] = inv
	log.Printf("[billing]    請求書を発行 invoice=%s amount=%s（決済手段は未参照）", inv.ID, inv.Amount)
	return s.bus.Publish(ctx, events.InvoiceIssued{
		InvoiceID:        inv.ID,
		BillingAccountID: ev.BillingAccountID,
		Amount:           ev.Amount,
	})
}

func (s *Service) onInvoicePaid(ctx context.Context, e shared.Event) error {
	ev := e.(events.InvoicePaid)
	if inv, ok := s.invoices[ev.InvoiceID]; ok {
		inv.MarkPaid()
		log.Printf("[billing]    請求書を入金済みに更新 invoice=%s status=%s", inv.ID, inv.Status)
	}
	return nil
}
