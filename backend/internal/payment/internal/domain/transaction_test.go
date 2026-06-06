package domain

import (
	"errors"
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
	if err := tx.MarkCaptured(); err != nil {
		t.Fatalf("MarkCaptured: %v", err)
	}
	if tx.Status != StatusCaptured {
		t.Errorf("Status = %q, want %q", tx.Status, StatusCaptured)
	}
}

func TestMarkFailed_RecordsReason(t *testing.T) {
	tx := NewTransaction("TXN-1", "INV-1", "PM-card-primary", shared.JPY(3000))
	if err := tx.MarkFailed("insufficient_funds"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if tx.Status != StatusFailed {
		t.Errorf("Status = %q, want %q", tx.Status, StatusFailed)
	}
	if tx.FailureReason != "insufficient_funds" {
		t.Errorf("FailureReason = %q, want insufficient_funds", tx.FailureReason)
	}
}

// 口座振替・払込票は requested → pending → captured と後日確定する。
func TestMarkPending_ThenConfirm(t *testing.T) {
	tx := NewTransaction("TXN-1", "INV-1", "PM-bank-transfer", shared.JPY(3000))
	if err := tx.MarkPending(); err != nil {
		t.Fatalf("MarkPending: %v", err)
	}
	if tx.Status != StatusPending || !tx.IsPending() {
		t.Errorf("Status = %q, want %q", tx.Status, StatusPending)
	}
	if err := tx.Confirm(); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if tx.Status != StatusCaptured {
		t.Errorf("Status = %q, want %q（後日確定）", tx.Status, StatusCaptured)
	}
}

// 状態機械は不正な遷移を弾く（確定済み/失敗からの再遷移、未保留からの確定を拒否）。
func TestTransaction_RejectsInvalidTransitions(t *testing.T) {
	t.Run("captured からは再遷移できない", func(t *testing.T) {
		tx := NewTransaction("TXN-1", "INV-1", "PM-card-secondary", shared.JPY(3000))
		_ = tx.MarkCaptured()
		if err := tx.MarkFailed("x"); !errors.Is(err, ErrNotRequested) {
			t.Errorf("MarkFailed err = %v, want ErrNotRequested", err)
		}
		if err := tx.MarkPending(); !errors.Is(err, ErrNotRequested) {
			t.Errorf("MarkPending err = %v, want ErrNotRequested", err)
		}
	})

	t.Run("requested からは Confirm できない", func(t *testing.T) {
		tx := NewTransaction("TXN-1", "INV-1", "PM-bank-transfer", shared.JPY(3000))
		if err := tx.Confirm(); !errors.Is(err, ErrNotPending) {
			t.Errorf("Confirm err = %v, want ErrNotPending", err)
		}
	})

	t.Run("failed からは確定できない", func(t *testing.T) {
		tx := NewTransaction("TXN-1", "INV-1", "PM-card-primary", shared.JPY(3000))
		_ = tx.MarkFailed("insufficient_funds")
		if err := tx.MarkCaptured(); !errors.Is(err, ErrNotRequested) {
			t.Errorf("MarkCaptured err = %v, want ErrNotRequested", err)
		}
	})
}

// 不変条件: 1 試行 = 1 手段 = 1 金額。状態がどう遷移しても手段・金額は固定される
// （手段を変えるなら新しい Transaction を作る）。
func TestTransaction_MethodAndAmountAreFixed(t *testing.T) {
	tx := NewTransaction("TXN-1", "INV-1", "PM-bank-transfer", shared.JPY(3000))
	method, amount := tx.PaymentMethod, tx.Amount

	_ = tx.MarkPending()
	_ = tx.Confirm()

	if tx.PaymentMethod != method {
		t.Errorf("PaymentMethod が変化した: %q → %q", method, tx.PaymentMethod)
	}
	if tx.Amount != amount {
		t.Errorf("Amount が変化した: %v → %v", amount, tx.Amount)
	}
}
