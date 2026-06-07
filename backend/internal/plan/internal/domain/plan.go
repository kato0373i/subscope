// Package domain は plan モジュールの集約・値オブジェクトを閉じ込める private ドメイン層。
// 依存は shared と標準ライブラリのみ。
package domain

import (
	"errors"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

// BillingInterval は課金周期。
type BillingInterval string

const (
	IntervalMonthly BillingInterval = "monthly"
	IntervalYearly  BillingInterval = "yearly"
)

func (i BillingInterval) valid() bool {
	return i == IntervalMonthly || i == IntervalYearly
}

var (
	ErrEmptyName        = errors.New("プラン名は必須です")
	ErrNonPositivePrice = errors.New("価格は 1 以上である必要があります")
	ErrInvalidInterval  = errors.New("課金周期が不正です")
)

// Price はプランの価格（金額 + 課金周期）を表す不変の値オブジェクト。
// 発行済 Invoice にはこの Price をスナップショットとして焼き込み、後のプラン改定の影響を受けないようにする。
type Price struct {
	Amount   shared.Money
	Interval BillingInterval
}

// NewPrice は価格 VO を生成する。金額は正、周期は既知の値のみ許可する。
func NewPrice(amount shared.Money, interval BillingInterval) (Price, error) {
	if amount.Amount <= 0 {
		return Price{}, ErrNonPositivePrice
	}
	if !interval.valid() {
		return Price{}, ErrInvalidInterval
	}
	return Price{Amount: amount, Interval: interval}, nil
}

// Plan は課金プランの集約。現在の価格を保持し、価格改定できる。
type Plan struct {
	ID    shared.PlanID
	OrgID shared.OrgID
	Name  string
	price Price
}

// New はプランを生成する。
func New(id shared.PlanID, orgID shared.OrgID, name string, price Price) (*Plan, error) {
	if name == "" {
		return nil, ErrEmptyName
	}
	return &Plan{ID: id, OrgID: orgID, Name: name, price: price}, nil
}

// Price は現在の価格を返す。
func (p *Plan) Price() Price { return p.price }

// ChangePrice はプラン価格を改定する。既に発行済の Invoice はスナップショットを保持するため影響を受けない。
// Price のフィールドは公開されており NewPrice を経由しない構築が可能なため、ここでも不変条件を検証する。
func (p *Plan) ChangePrice(price Price) error {
	if price.Amount.Amount <= 0 {
		return ErrNonPositivePrice
	}
	if !price.Interval.valid() {
		return ErrInvalidInterval
	}
	p.price = price
	return nil
}

// Snapshot は Invoice へ焼き込むための現在価格のスナップショットを返す。
// Price は値型のため、返した値は以降のプラン改定から独立している。
func (p *Plan) Snapshot() Price { return p.price }
