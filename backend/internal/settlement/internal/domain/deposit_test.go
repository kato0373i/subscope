package domain

import (
	"testing"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

func TestBankDeposit_AllocateAndRemaining(t *testing.T) {
	d := NewBankDeposit("DEP-1", "REF-1", "BA-1", "ヤマダ", shared.JPY(5000))
	if err := d.Allocate("INV-1", shared.JPY(2000)); err != nil {
		t.Fatalf("Allocate#1: %v", err)
	}
	if err := d.Allocate("INV-2", shared.JPY(3000)); err != nil {
		t.Fatalf("Allocate#2: %v", err)
	}
	if !d.FullyApplied() {
		t.Error("全額充当後は FullyApplied であるべき")
	}
	if d.Remaining().Amount != 0 {
		t.Errorf("Remaining = %d, want 0", d.Remaining().Amount)
	}
	if len(d.Allocations()) != 2 {
		t.Errorf("Allocations = %d, want 2", len(d.Allocations()))
	}
}

// 入金額を超える充当（過充当）は弾く：Σ充当額 ≤ 入金額。
func TestBankDeposit_RejectsOverApplication(t *testing.T) {
	d := NewBankDeposit("DEP-1", "REF-1", "BA-1", "ヤマダ", shared.JPY(3000))
	if err := d.Allocate("INV-1", shared.JPY(2000)); err != nil {
		t.Fatalf("Allocate#1: %v", err)
	}
	if err := d.Allocate("INV-2", shared.JPY(1500)); err != ErrOverApplication {
		t.Fatalf("過充当は ErrOverApplication を返すべき: got %v", err)
	}
	// 拒否された分は累計に反映されない。
	if d.Applied().Amount != 2000 {
		t.Errorf("Applied = %d, want 2000（過充当分は加算しない）", d.Applied().Amount)
	}
}

func TestBankDeposit_RejectsCurrencyMismatch(t *testing.T) {
	d := NewBankDeposit("DEP-1", "REF-1", "BA-1", "ヤマダ", shared.JPY(3000))
	if err := d.Allocate("INV-1", shared.Money{Amount: 1000, Currency: "USD"}); err != ErrCurrencyMismatch {
		t.Fatalf("通貨不一致は ErrCurrencyMismatch を返すべき: got %v", err)
	}
}
