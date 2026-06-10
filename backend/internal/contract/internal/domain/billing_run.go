package domain

import (
	"time"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

// BillingItem は Billing Run が抽出した 1 契約ぶんの請求予定。
// 「誰に・いくら・どの期間を請求するか」だけを持ち、決済手段への参照を一切含まない。
// これは設計の背骨「債権 ≠ 決済手段」の現れ：請求の起票は手段を知らないまま完結し、
// 手段は後段（collection）で遅延束縛される。ゆえに手段の付け替えは請求側に影響しない。
type BillingItem struct {
	ContractID       shared.ContractID
	BillingAccountID shared.BillingAccountID
	Amount           shared.Money
	Period           string
}

// BillingRun は指定時点（asOf）で請求サイクルが到来した契約を抽出する純粋なプランナ。
// 副作用（イベント発行・状態変更）を持たないため、そのままドライラン（プレビュー）に使える。
// 実際の発火・次回請求日の前進は application 層（contract.Service）の責務。
type BillingRun struct {
	asOf time.Time
}

// NewBillingRun は asOf 時点の請求対象を抽出する Run を生成する。
func NewBillingRun(asOf time.Time) BillingRun {
	return BillingRun{asOf: asOf}
}

// ID は asOf から決まる安定した実行 ID（同一日付の Run は同一 ID）。監査・トレース用。
func (r BillingRun) ID() shared.BillingRunID {
	return shared.BillingRunID("BR-" + r.asOf.Format("20060102"))
}

// Plan は課金対象かつ次回請求日が asOf 以前の契約を抽出し、請求予定を返す。
// 1 契約につき 1 件（最大 1 期間）のみ。複数期間の遡及キャッチアップは行わない。
// 決定的順序を保つため呼び出し側がソート済みのスライスを渡す前提。
func (r BillingRun) Plan(contracts []*Contract) []BillingItem {
	items := make([]BillingItem, 0, len(contracts))
	for _, c := range contracts {
		if !c.IsDue(r.asOf) {
			continue
		}
		items = append(items, BillingItem{
			ContractID:       c.ID,
			BillingAccountID: c.BillingAccountID,
			Amount:           c.MonthlyFee,
			Period:           c.CurrentBillingPeriod(),
		})
	}
	return items
}
