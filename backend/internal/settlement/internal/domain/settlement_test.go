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
