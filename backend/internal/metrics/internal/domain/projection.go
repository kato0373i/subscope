// Package domain は metrics モジュールの投影（Read Model）を閉じ込める private ドメイン層。
// 依存は shared と標準ライブラリのみ。書き込み集約ではなく、イベントを集計する CQRS の読み取り側。
package domain

import "github.com/kato0373i/subscope/backend/internal/shared"

// Projection は各統合イベントを集計した読み取り専用の指標。
// 状態は単調に更新され、ビジネス上の「現在値」をその場で答えるためのスナップショット元。
type Projection struct {
	ActiveContracts  int          // 有効契約数（在籍中の契約数）
	ChurnedContracts int          // 解約された契約数（累計）
	InvoicesIssued   int          // 発行済み請求書の件数
	InvoicesPaid     int          // 全額消込が完了した請求書の件数
	BilledTotal      shared.Money // 請求総額（InvoiceIssued の Amount 合計）
	RecoveredTotal   shared.Money // 未収から回収できた金額の合計
	WrittenOffTotal  shared.Money // 貸倒として落とした金額の合計
	RefundTotal      shared.Money // 赤伝（返金）の金額合計

	// active は在籍中の契約 ID 集合。重複 Activated や順不同/重複の Cancelled で
	// ActiveContracts が壊れない（負値にならない）よう ContractID 単位で遷移を検証するための非公開状態。
	// 非公開のため Snapshot（値コピー）の利用者からは見えず、現在値は ActiveContracts(int) で固定される。
	active map[shared.ContractID]struct{}
}

// OnContractActivated は契約有効化を反映する。既に在籍中の ID は二重計上しない。
func (p *Projection) OnContractActivated(id shared.ContractID) {
	if p.active == nil {
		p.active = make(map[shared.ContractID]struct{})
	}
	if _, ok := p.active[id]; ok {
		return // 重複 Activated は無視
	}
	p.active[id] = struct{}{}
	p.ActiveContracts++
}

// OnContractCancelled は契約解約を反映する（チャーン）。
// 在籍していない ID の取消は no-op とし、ActiveContracts が負値になるのを防ぐ。
func (p *Projection) OnContractCancelled(id shared.ContractID) {
	if _, ok := p.active[id]; !ok {
		return // 未在籍（重複/順不同の Cancelled）は無視
	}
	delete(p.active, id)
	p.ActiveContracts--
	p.ChurnedContracts++
}

// OnInvoiceIssued は請求書発行を反映する。
// 金額の加算を先に確定させ、成功した場合のみ件数を増やして集計の整合を保つ。
func (p *Projection) OnInvoiceIssued(amount shared.Money) error {
	if err := accumulate(&p.BilledTotal, amount); err != nil {
		return err
	}
	p.InvoicesIssued++
	return nil
}

// OnInvoicePaid は請求書の全額消込を反映する。
func (p *Projection) OnInvoicePaid() { p.InvoicesPaid++ }

// OnCollectionRecovered は未収案件の回収を反映する。
func (p *Projection) OnCollectionRecovered(amount shared.Money) error {
	return accumulate(&p.RecoveredTotal, amount)
}

// OnCollectionWrittenOff は貸倒計上を反映する。
func (p *Projection) OnCollectionWrittenOff(amount shared.Money) error {
	return accumulate(&p.WrittenOffTotal, amount)
}

// OnCreditNoteIssued は赤伝（返金）発行を反映する。
func (p *Projection) OnCreditNoteIssued(amount shared.Money) error {
	return accumulate(&p.RefundTotal, amount)
}

// accumulate は Money を安全に加算する。ゼロ値（通貨未設定）の累計器には
// 最初の通貨を採用し、以降は Money.Add（通貨不一致・オーバーフロー検出）に委ねる。
func accumulate(acc *shared.Money, delta shared.Money) error {
	if acc.Currency == "" {
		*acc = delta
		return nil
	}
	sum, err := acc.Add(delta)
	if err != nil {
		return err
	}
	*acc = sum
	return nil
}
