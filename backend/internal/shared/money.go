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

// Add は同一通貨同士の加算を行う。通貨が異なる場合はエラーを返す。
func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("通貨が異なるため加算できません: %s + %s", m.Currency, other.Currency)
	}
	sum := m.Amount + other.Amount
	// int64 の桁あふれを検出し、壊れた金額を返さない。
	if (other.Amount > 0 && sum < m.Amount) || (other.Amount < 0 && sum > m.Amount) {
		return Money{}, fmt.Errorf("金額がオーバーフローしました: %d + %d", m.Amount, other.Amount)
	}
	return Money{Amount: sum, Currency: m.Currency}, nil
}

// IsZero は金額がゼロかを返す。
func (m Money) IsZero() bool { return m.Amount == 0 }

// IsNegative は負の金額かを返す（返金・赤伝の判定に用いる）。
func (m Money) IsNegative() bool { return m.Amount < 0 }
