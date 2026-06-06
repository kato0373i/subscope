package settlement_test

import (
	"context"
	"testing"

	"github.com/kato0373i/subscope/backend/internal/platform/eventbus"
	"github.com/kato0373i/subscope/backend/internal/settlement"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// クレカ即時入金（PaymentSucceeded）を受けて消込し、InvoicePaid を発行する。
func TestService_PaymentSucceededSettlesInvoice(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = settlement.NewService(bus)

	var got *events.InvoicePaid
	bus.Subscribe(events.NameInvoicePaid, func(_ context.Context, e shared.Event) error {
		ev := e.(events.InvoicePaid)
		got = &ev
		return nil
	})

	err := bus.Publish(context.Background(), events.PaymentSucceeded{
		InvoiceID:     "INV-1",
		TransactionID: "TXN-1",
		Amount:        shared.JPY(3000),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if got == nil {
		t.Fatal("InvoicePaid が発行されなかった")
	}
	if got.InvoiceID != "INV-1" {
		t.Errorf("InvoiceID = %q, want INV-1", got.InvoiceID)
	}
}

// 同一の入金（同じ TransactionID）が二重通知されても、消込は一度だけ・InvoicePaid も一度だけ。
func TestService_SettlementIsIdempotent(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = settlement.NewService(bus)

	var paid int
	bus.Subscribe(events.NameInvoicePaid, func(context.Context, shared.Event) error { paid++; return nil })

	succeeded := events.PaymentSucceeded{
		InvoiceID:     "INV-1",
		TransactionID: "TXN-1",
		Amount:        shared.JPY(3000),
	}
	// PSP の二重通知を模擬。
	if err := bus.Publish(context.Background(), succeeded); err != nil {
		t.Fatalf("Publish#1: %v", err)
	}
	if err := bus.Publish(context.Background(), succeeded); err != nil {
		t.Fatalf("Publish#2: %v", err)
	}

	if paid != 1 {
		t.Errorf("InvoicePaid = %d, want 1（二重消込を抑止）", paid)
	}
}

// 銀行入金の取込：請求先 ID ＋ 金額で自動消込し InvoicePaid を発行する。
func TestService_ImportBankDepositAutoSettles(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := settlement.NewService(bus)

	var paid, partial, unmatched int
	bus.Subscribe(events.NameInvoicePaid, func(context.Context, shared.Event) error { paid++; return nil })
	bus.Subscribe(events.NameInvoicePartiallyPaid, func(context.Context, shared.Event) error { partial++; return nil })
	bus.Subscribe(events.NameUnmatchedDepositDetected, func(context.Context, shared.Event) error { unmatched++; return nil })

	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})

	if err := s.ImportBankDeposits(context.Background(), []settlement.DepositInput{
		{Reference: "REF-1", Account: "BA-1", PayerName: "ヤマダ", Amount: shared.JPY(3000)},
	}); err != nil {
		t.Fatalf("ImportBankDeposits: %v", err)
	}

	if paid != 1 || partial != 0 || unmatched != 0 {
		t.Errorf("paid=%d partial=%d unmatched=%d, want 1/0/0", paid, partial, unmatched)
	}
}

// 団体一括の戻し込み：1 入金を同一請求先の複数請求へ按分し、それぞれ全額消込。
func TestService_ImportBankDepositApportions(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := settlement.NewService(bus)

	var paid int
	bus.Subscribe(events.NameInvoicePaid, func(context.Context, shared.Event) error { paid++; return nil })

	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(2000)})
	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-2", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})

	if err := s.ImportBankDeposits(context.Background(), []settlement.DepositInput{
		{Reference: "REF-1", Account: "BA-1", PayerName: "協会", Amount: shared.JPY(5000)},
	}); err != nil {
		t.Fatalf("ImportBankDeposits: %v", err)
	}

	if paid != 2 {
		t.Errorf("InvoicePaid = %d, want 2（2 請求へ按分・全額消込）", paid)
	}
}

// 金額が一致しない入金は UnmatchedDepositDetected を発行（自動消込しない）。
func TestService_ImportBankDepositUnmatched(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := settlement.NewService(bus)

	var paid, unmatched int
	bus.Subscribe(events.NameInvoicePaid, func(context.Context, shared.Event) error { paid++; return nil })
	bus.Subscribe(events.NameUnmatchedDepositDetected, func(context.Context, shared.Event) error { unmatched++; return nil })

	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})

	if err := s.ImportBankDeposits(context.Background(), []settlement.DepositInput{
		{Reference: "REF-1", Account: "BA-1", PayerName: "ヤマダ", Amount: shared.JPY(2500)},
	}); err != nil {
		t.Fatalf("ImportBankDeposits: %v", err)
	}

	if paid != 0 || unmatched != 1 {
		t.Errorf("paid=%d unmatched=%d, want 0/1", paid, unmatched)
	}
}

// 同一参照番号の入金は二重取込しても一度だけ処理する（冪等）。
func TestService_ImportBankDepositIdempotent(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := settlement.NewService(bus)

	var paid int
	bus.Subscribe(events.NameInvoicePaid, func(context.Context, shared.Event) error { paid++; return nil })

	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})

	dep := []settlement.DepositInput{{Reference: "REF-1", Account: "BA-1", PayerName: "ヤマダ", Amount: shared.JPY(3000)}}
	mustImport(t, s, dep)
	mustImport(t, s, dep) // 二重取込

	if paid != 1 {
		t.Errorf("InvoicePaid = %d, want 1（二重取込を抑止）", paid)
	}
}

// 手動消込：部分入金を充当すると InvoicePartiallyPaid、残額を充当すると InvoicePaid。
func TestService_ReconcileManuallyPartialThenFull(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := settlement.NewService(bus)

	var paid, partial int
	bus.Subscribe(events.NameInvoicePaid, func(context.Context, shared.Event) error { paid++; return nil })
	bus.Subscribe(events.NameInvoicePartiallyPaid, func(context.Context, shared.Event) error { partial++; return nil })

	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})

	if err := s.ReconcileManually(context.Background(), "INV-1", shared.JPY(1000)); err != nil {
		t.Fatalf("ReconcileManually#1: %v", err)
	}
	if err := s.ReconcileManually(context.Background(), "INV-1", shared.JPY(2000)); err != nil {
		t.Fatalf("ReconcileManually#2: %v", err)
	}

	if partial != 1 {
		t.Errorf("InvoicePartiallyPaid = %d, want 1", partial)
	}
	if paid != 1 {
		t.Errorf("InvoicePaid = %d, want 1", paid)
	}
}

// 手動消込は残額を超える充当（過消込）を弾く。
func TestService_ReconcileManuallyRejectsOverpay(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := settlement.NewService(bus)

	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})

	if err := s.ReconcileManually(context.Background(), "INV-1", shared.JPY(4000)); err == nil {
		t.Error("残額超過の手動消込はエラーになるべき")
	}
}

func mustPublish(t *testing.T, bus shared.EventBus, e shared.Event) {
	t.Helper()
	if err := bus.Publish(context.Background(), e); err != nil {
		t.Fatalf("Publish(%s): %v", e.EventName(), err)
	}
}

func mustImport(t *testing.T, s *settlement.Service, deps []settlement.DepositInput) {
	t.Helper()
	if err := s.ImportBankDeposits(context.Background(), deps); err != nil {
		t.Fatalf("ImportBankDeposits: %v", err)
	}
}
