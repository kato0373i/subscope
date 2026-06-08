package domain

import (
	"testing"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

func TestProjection_AggregatesLifecycle(t *testing.T) {
	var p Projection

	p.OnContractActivated()
	p.OnContractActivated()
	p.OnContractCancelled() // 1 件解約

	if p.ActiveContracts != 1 {
		t.Errorf("ActiveContracts = %d, want 1", p.ActiveContracts)
	}
	if p.ChurnedContracts != 1 {
		t.Errorf("ChurnedContracts = %d, want 1", p.ChurnedContracts)
	}
}

func TestProjection_AggregatesMoney(t *testing.T) {
	var p Projection

	if err := p.OnInvoiceIssued(shared.JPY(3000)); err != nil {
		t.Fatalf("OnInvoiceIssued: %v", err)
	}
	if err := p.OnInvoiceIssued(shared.JPY(2000)); err != nil {
		t.Fatalf("OnInvoiceIssued: %v", err)
	}
	p.OnInvoicePaid()
	if err := p.OnCollectionRecovered(shared.JPY(1500)); err != nil {
		t.Fatalf("OnCollectionRecovered: %v", err)
	}
	if err := p.OnCreditNoteIssued(shared.JPY(500)); err != nil {
		t.Fatalf("OnCreditNoteIssued: %v", err)
	}

	if p.InvoicesIssued != 2 {
		t.Errorf("InvoicesIssued = %d, want 2", p.InvoicesIssued)
	}
	if p.BilledTotal.Amount != 5000 {
		t.Errorf("BilledTotal = %d, want 5000", p.BilledTotal.Amount)
	}
	if p.InvoicesPaid != 1 {
		t.Errorf("InvoicesPaid = %d, want 1", p.InvoicesPaid)
	}
	if p.RecoveredTotal.Amount != 1500 {
		t.Errorf("RecoveredTotal = %d, want 1500", p.RecoveredTotal.Amount)
	}
	if p.RefundTotal.Amount != 500 {
		t.Errorf("RefundTotal = %d, want 500", p.RefundTotal.Amount)
	}
}

// 通貨不一致は Money.Add で弾かれ、accumulate がエラーを伝播する。
func TestProjection_RejectsMixedCurrency(t *testing.T) {
	var p Projection
	if err := p.OnInvoiceIssued(shared.JPY(1000)); err != nil {
		t.Fatalf("OnInvoiceIssued: %v", err)
	}
	if err := p.OnInvoiceIssued(shared.Money{Amount: 10, Currency: "USD"}); err == nil {
		t.Error("通貨不一致はエラーになるべき")
	}
}
