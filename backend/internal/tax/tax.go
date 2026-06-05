// Package tax は税モジュールの公開 API。
// 消費税（インボイス制度）の計算と適格請求書発行事業者の登録番号管理を提供する。
// billing が請求確定時に Calculate を呼び、得た Breakdown を Invoice に焼き込む想定。
package tax

import (
	"time"

	"github.com/kato0373i/subscope/backend/internal/tax/internal/domain"
)

// 内部ドメイン型を型エイリアスで再エクスポートし、外部からは tax.* で参照させる。
type (
	Category     = domain.Category
	RoundingMode = domain.RoundingMode
	Line         = domain.Line
	CategoryTax  = domain.CategoryTax
	Breakdown    = domain.Breakdown
	Registration = domain.Registration
)

const (
	CategoryStandard = domain.CategoryStandard
	CategoryReduced  = domain.CategoryReduced
	CategoryExempt   = domain.CategoryExempt

	RoundFloor  = domain.RoundFloor
	RoundCeil   = domain.RoundCeil
	RoundHalfUp = domain.RoundHalfUp
)

// Service は税計算ドメインサービスのファサード。
type Service struct {
	calc domain.Calculator
}

// NewService は端数処理方法を指定して Service を生成する。
func NewService(mode RoundingMode) *Service {
	return &Service{calc: domain.NewCalculator(mode)}
}

// Calculate は明細群から税率別内訳を算出する。
func (s *Service) Calculate(lines []Line) Breakdown {
	return s.calc.Calculate(lines)
}

// NewRegistration は登録番号を検証して Registration を生成する。
func NewRegistration(number string, validFrom time.Time) (Registration, error) {
	return domain.NewRegistration(number, validFrom)
}
