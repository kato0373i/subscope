// Command api は subscope バックエンドのエントリポイント。
// 現状は最小スライス（請求〜回収フロー）をインメモリで実演するデモ。
package main

import (
	"context"
	"log"

	"github.com/kato0373i/subscope/backend/internal/billing"
	"github.com/kato0373i/subscope/backend/internal/billingaccount"
	"github.com/kato0373i/subscope/backend/internal/collection"
	"github.com/kato0373i/subscope/backend/internal/contract"
	"github.com/kato0373i/subscope/backend/internal/coupon"
	"github.com/kato0373i/subscope/backend/internal/creditnote"
	"github.com/kato0373i/subscope/backend/internal/dunning"
	"github.com/kato0373i/subscope/backend/internal/member"
	"github.com/kato0373i/subscope/backend/internal/notification"
	"github.com/kato0373i/subscope/backend/internal/organization"
	"github.com/kato0373i/subscope/backend/internal/payment"
	"github.com/kato0373i/subscope/backend/internal/paymentmethod"
	"github.com/kato0373i/subscope/backend/internal/plan"
	"github.com/kato0373i/subscope/backend/internal/platform/eventbus"
	"github.com/kato0373i/subscope/backend/internal/settlement"
	"github.com/kato0373i/subscope/backend/internal/shared"
)

func main() {
	log.SetFlags(0)
	bus := eventbus.NewInMemory()

	// 各モジュールはイベントバスだけを介して結線される（直接の関数呼び出しはしない）。
	orgs := organization.NewService()
	members := member.NewService()
	accounts := billingaccount.NewService()
	plans := plan.NewService()
	coupons := coupon.NewService()
	pms := paymentmethod.NewService(bus)
	contracts := contract.NewService(bus)
	_ = billing.NewService(bus)
	_ = collection.NewService(bus)
	_ = payment.NewService(bus)
	_ = settlement.NewService(bus)
	_ = dunning.NewService(bus)
	_ = notification.NewService(bus)
	_ = creditnote.NewService(bus)

	ctx := context.Background()

	// デモ: テナント・会員・請求先を設定する。
	if err := orgs.Register("ORG-0001", "サンプル協会"); err != nil {
		log.Fatalf("orgs.Register: %v", err)
	}
	if err := members.Register("MEM-0001", "ORG-0001", "山田 太郎", "yamada@example.com"); err != nil {
		log.Fatalf("members.Register: %v", err)
	}
	if err := accounts.Register("BA-0001", "ORG-0001", "山田 太郎"); err != nil {
		log.Fatalf("accounts.Register: %v", err)
	}
	if err := accounts.AddMember("BA-0001", "MEM-0001"); err != nil {
		log.Fatalf("accounts.AddMember: %v", err)
	}

	// デモ: 決済手段を登録する（クレカ2枚 + 払込票）。
	if err := pms.RegisterCreditCard(ctx, "PM-card-primary", "BA-0001", "tok_visa_001", 1); err != nil {
		log.Fatalf("RegisterCreditCard: %v", err)
	}
	if err := pms.RegisterCreditCard(ctx, "PM-card-secondary", "BA-0001", "tok_mc_002", 2); err != nil {
		log.Fatalf("RegisterCreditCard: %v", err)
	}
	if err := pms.RegisterPaymentSlip(ctx, "PM-payment-slip", "BA-0001", 4); err != nil {
		log.Fatalf("RegisterPaymentSlip: %v", err)
	}

	// デモ: 口座振替を登録して審査を通過させる（pending → reviewing → completed）。
	if err := pms.RegisterBankAccount(ctx, "PM-bank-transfer", "BA-0001", "tok_bank_003", 3); err != nil {
		log.Fatalf("RegisterBankAccount: %v", err)
	}
	if err := pms.StartBankAccountReview(ctx, "PM-bank-transfer"); err != nil {
		log.Fatalf("StartBankAccountReview: %v", err)
	}
	if err := pms.CompleteBankAccountRegistration(ctx, "PM-bank-transfer"); err != nil {
		log.Fatalf("CompleteBankAccountRegistration: %v", err)
	}

	// デモ: 月会費プランとクーポンを登録し、割引適用を確認する。
	price, err := plan.NewPrice(shared.JPY(3000), plan.IntervalMonthly)
	if err != nil {
		log.Fatalf("plan.NewPrice: %v", err)
	}
	if err := plans.Register("PLAN-0001", "ORG-0001", "月会費", price); err != nil {
		log.Fatalf("plans.Register: %v", err)
	}
	if err := coupons.Register("CPN-0001", "ORG-0001", "WELCOME", coupon.DiscountPercent, 10, 100); err != nil {
		log.Fatalf("coupons.Register: %v", err)
	}
	if err := coupons.Redeem("CPN-0001", "BA-0001"); err != nil {
		log.Fatalf("coupons.Redeem: %v", err)
	}
	snapshot, err := plans.Snapshot("PLAN-0001")
	if err != nil {
		log.Fatalf("plans.Snapshot: %v", err)
	}
	discounted, err := coupons.Apply("CPN-0001", snapshot.Amount)
	if err != nil {
		log.Fatalf("coupons.Apply: %v", err)
	}
	log.Printf("[demo] クーポン適用: %s → %s", snapshot.Amount, discounted)

	// デモ: 月会費 3,000 円の契約を登録する。
	contracts.RegisterContract("CT-0001", "MEM-0001", "BA-0001", shared.JPY(3000))

	log.Println("=== 請求〜回収フローを実行 ===")
	if err := contracts.TriggerBilling(ctx, "CT-0001"); err != nil {
		log.Fatalf("error: %v", err)
	}
	log.Println("=== 完了 ===")
}
