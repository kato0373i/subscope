// Package domain は metrics モジュールの投影（Read Model）を閉じ込める private ドメイン層。
// 依存は shared と標準ライブラリのみ。書き込み集約ではなく、イベントを集計する CQRS の読み取り側。
package domain

import "github.com/kato0373i/subscope/backend/internal/shared"

// Projection は各統合イベントを集計した読み取り専用の指標。
// 状態は単調に更新され、ビジネス上の「現在値」をその場で答えるためのスナップショット元。
type Projection struct {
	ActiveContracts  int          // 有効契約数（Activated で +1、Cancelled で -1）
	ChurnedContracts int          // 解約された契約数（累計）
	InvoicesIssued   int          // 発行済み請求書の件数
	InvoicesPaid     int          // 全額消込が完了した請求書の件数
	BilledTotal      shared.Money // 請求総額（InvoiceIssued の Amount 合計）
	RecoveredTotal   shared.Money // 未収から回収できた金額の合計
	WrittenOffTotal  shared.Money // 貸倒として落とした金額の合計
	RefundTotal      shared.Money // 赤伝（返金）の金額合計
}

// OnContractActivated は契約有効化を反映する。
func (p *Projection) OnContractActivated() { p.ActiveContracts++ }

// OnContractCancelled は契約解約を反映する（チャーン）。
func (p *Projection) OnContractCancelled() {
	p.ActiveContracts--
	p.ChurnedContracts++
}

// OnInvoiceIssued は請求書発行を反映する。
func (p *Projection) OnInvoiceIssued(amount shared.Money) error {
	p.InvoicesIssued++
	return accumulate(&p.BilledTotal, amount)
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
