package domain

import (
	"testing"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

func TestNew_BindsDepositToInvoice(t *testing.T) {
	st := New("STL-1", "INV-1", shared.JPY(3000))

	if st.ID != "STL-1" {
		t.Errorf("ID = %q, want STL-1", st.ID)
	}
	if st.Invoice != "INV-1" {
		t.Errorf("Invoice = %q, want INV-1", st.Invoice)
	}
	if st.Amount.Amount != 3000 {
		t.Errorf("Amount = %v, want 3000", st.Amount)
	}
}

func TestReconcile_FullAmount(t *testing.T) {
	st := New("STL-1", "INV-1", shared.JPY(3000))
	if err := st.Reconcile(shared.JPY(3000)); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if !st.FullyReconciled() {
		t.Error("全額充当後は FullyReconciled であるべき")
	}
}

// 入金額を超える消込（過消込）は弾く：Σ消込額 ≤ 入金額。
func TestReconcile_RejectsOverReconciliation(t *testing.T) {
	st := New("STL-1", "INV-1", shared.JPY(3000))
	if err := st.Reconcile(shared.JPY(2000)); err != nil {
		t.Fatalf("1 回目の Reconcile: %v", err)
	}
	// 残り 1000 に対し 1500 を充当しようとすると過消込でエラー。
	if err := st.Reconcile(shared.JPY(1500)); err != ErrOverReconciliation {
		t.Fatalf("過消込は ErrOverReconciliation を返すべき: got %v", err)
	}
	// 拒否された分は累計に反映されない。
	if st.Reconciled().Amount != 2000 {
		t.Errorf("Reconciled = %d, want 2000（過消込分は加算しない）", st.Reconciled().Amount)
	}
}
