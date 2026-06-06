package domain

import (
	"testing"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

func TestNewTransaction_StartsRequested(t *testing.T) {
	tx := NewTransaction("TXN-1", "INV-1", "PM-card-primary", shared.JPY(3000))

	if tx.Status != StatusRequested {
		t.Errorf("Status = %q, want %q", tx.Status, StatusRequested)
	}
	// invoice_id と payment_method_id がここで初めて出会う。
	if tx.Invoice != "INV-1" || tx.PaymentMethod != "PM-card-primary" {
		t.Errorf("Invoice/Method = %q/%q", tx.Invoice, tx.PaymentMethod)
	}
}

func TestMarkCaptured(t *testing.T) {
	tx := NewTransaction("TXN-1", "INV-1", "PM-card-primary", shared.JPY(3000))
	tx.MarkCaptured()
	if tx.Status != StatusCaptured {
		t.Errorf("Status = %q, want %q", tx.Status, StatusCaptured)
	}
}

func TestMarkFailed_RecordsReason(t *testing.T) {
	tx := NewTransaction("TXN-1", "INV-1", "PM-card-primary", shared.JPY(3000))
	tx.MarkFailed("insufficient_funds")
	if tx.Status != StatusFailed {
		t.Errorf("Status = %q, want %q", tx.Status, StatusFailed)
	}
	if tx.FailureReason != "insufficient_funds" {
		t.Errorf("FailureReason = %q, want insufficient_funds", tx.FailureReason)
	}
}
