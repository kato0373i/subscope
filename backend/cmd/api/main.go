// Command api は subscope バックエンドのエントリポイント。
// 現状は最小スライス（請求〜回収フロー）をインメモリで実演するデモ。
package main

import (
	"context"
	"log"

	"github.com/kato0373i/subscope/backend/internal/billing"
	"github.com/kato0373i/subscope/backend/internal/collection"
	"github.com/kato0373i/subscope/backend/internal/contract"
	"github.com/kato0373i/subscope/backend/internal/payment"
	"github.com/kato0373i/subscope/backend/internal/platform/eventbus"
	"github.com/kato0373i/subscope/backend/internal/settlement"
	"github.com/kato0373i/subscope/backend/internal/shared"
)

func main() {
	log.SetFlags(0)
	bus := eventbus.NewInMemory()

	// 各モジュールはイベントバスだけを介して結線される（直接の関数呼び出しはしない）。
	contracts := contract.NewService(bus)
	_ = billing.NewService(bus)
	_ = collection.NewService(bus)
	_ = payment.NewService(bus)
	_ = settlement.NewService(bus)

	// デモ用の契約を 1 件登録（月会費 3,000 円）。
	contracts.RegisterContract("CT-0001", "MEM-0001", "BA-0001", shared.JPY(3000))

	log.Println("=== 請求〜回収フローを実行 ===")
	if err := contracts.TriggerBilling(context.Background(), "CT-0001"); err != nil {
		log.Fatalf("error: %v", err)
	}
	log.Println("=== 完了 ===")
}
