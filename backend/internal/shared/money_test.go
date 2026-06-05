package shared

import "testing"

func TestMoney_Add_SameCurrency(t *testing.T) {
	got, err := JPY(1000).Add(JPY(500))
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if got.Amount != 1500 || got.Currency != "JPY" {
		t.Errorf("Add = %v, want 1500 JPY", got)
	}
}

func TestMoney_Add_CurrencyMismatch(t *testing.T) {
	usd := Money{Amount: 100, Currency: "USD"}
	if _, err := JPY(1000).Add(usd); err == nil {
		t.Error("通貨不一致でエラーになるべき")
	}
}

func TestMoney_IsZeroAndNegative(t *testing.T) {
	if !JPY(0).IsZero() {
		t.Error("0 は IsZero であるべき")
	}
	if JPY(1).IsZero() {
		t.Error("1 は IsZero でないべき")
	}
	if !JPY(-1).IsNegative() {
		t.Error("-1 は IsNegative であるべき")
	}
	if JPY(1).IsNegative() {
		t.Error("1 は IsNegative でないべき")
	}
}
