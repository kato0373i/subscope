// Package domain は tax モジュールの内部ドメイン。
// 消費税（インボイス制度）の計算ポリシーと適格請求書発行事業者の登録番号を扱う。
package domain

import "github.com/kato0373i/subscope/backend/internal/shared"

// Category は税区分。日本の消費税を前提とする。
type Category string

const (
	CategoryStandard Category = "standard" // 標準税率 10%
	CategoryReduced  Category = "reduced"  // 軽減税率 8%
	CategoryExempt   Category = "exempt"   // 非課税・不課税・免税
)

// basisPoints は税率をベーシスポイント（1% = 100bp）で返す。
// 税率は本来は時期によって変わる政策値だが、PoC では固定値とする。
func (c Category) basisPoints() int64 {
	switch c {
	case CategoryStandard:
		return 1000
	case CategoryReduced:
		return 800
	default:
		return 0
	}
}

// RoundingMode は税額の端数処理方法。事業者が選択できる。
type RoundingMode string

const (
	RoundFloor  RoundingMode = "floor"   // 切り捨て（日本で最も一般的）
	RoundCeil   RoundingMode = "ceil"    // 切り上げ
	RoundHalfUp RoundingMode = "half_up" // 四捨五入
)

// Line は税計算の入力となる1明細（税抜金額と税区分）。
type Line struct {
	Category  Category
	NetAmount shared.Money // 税抜金額
}

// CategoryTax は税区分ごとの課税標準額と税額。
type CategoryTax struct {
	Category    Category
	TaxableBase int64 // 税抜合計
	TaxAmount   int64
}

// Breakdown は税率別の内訳。適格請求書の法的要件（税率別の金額・税額）を満たす。
type Breakdown struct {
	Currency   string
	ByCategory []CategoryTax
}

func (b Breakdown) TotalNet() int64 {
	var sum int64
	for _, c := range b.ByCategory {
		sum += c.TaxableBase
	}
	return sum
}

func (b Breakdown) TotalTax() int64 {
	var sum int64
	for _, c := range b.ByCategory {
		sum += c.TaxAmount
	}
	return sum
}

func (b Breakdown) TotalGross() int64 { return b.TotalNet() + b.TotalTax() }

// Calculator は税計算ポリシー（ドメインサービス）。
type Calculator struct {
	mode RoundingMode
}

func NewCalculator(mode RoundingMode) Calculator { return Calculator{mode: mode} }

// Calculate は明細群から税率別内訳を算出する。
//
// インボイス制度の重要ルール：端数処理は「税率ごとに1回」だけ行う。
// そのため明細ごとに丸めるのではなく、税区分ごとに税抜額を合算してから一度だけ丸める。
// 入力の税抜金額は非負（請求）を前提とする。返金（赤伝）は別途扱う。
func (c Calculator) Calculate(lines []Line) Breakdown {
	order := []Category{CategoryStandard, CategoryReduced, CategoryExempt}
	base := make(map[Category]int64, len(order))
	currency := ""
	for _, l := range lines {
		base[l.Category] += l.NetAmount.Amount
		if currency == "" {
			currency = l.NetAmount.Currency
		}
	}

	var by []CategoryTax
	for _, cat := range order {
		net, ok := base[cat]
		if !ok {
			continue
		}
		tax := round(net*cat.basisPoints(), 10000, c.mode)
		by = append(by, CategoryTax{Category: cat, TaxableBase: net, TaxAmount: tax})
	}
	return Breakdown{Currency: currency, ByCategory: by}
}

// round は numerator/denominator を指定モードで整数に丸める（非負前提）。
func round(numerator, denominator int64, mode RoundingMode) int64 {
	q := numerator / denominator
	r := numerator % denominator
	if r == 0 {
		return q
	}
	switch mode {
	case RoundCeil:
		return q + 1
	case RoundHalfUp:
		if r*2 >= denominator {
			return q + 1
		}
		return q
	default: // RoundFloor
		return q
	}
}
