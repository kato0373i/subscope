package shared

import "fmt"

// Money は最小通貨単位の金額。日本円を前提に整数で保持し、浮動小数の誤差を避ける。
type Money struct {
	Amount   int64
	Currency string
}

// JPY は日本円の金額を生成する。
func JPY(amount int64) Money { return Money{Amount: amount, Currency: "JPY"} }

func (m Money) String() string { return fmt.Sprintf("%d %s", m.Amount, m.Currency) }
