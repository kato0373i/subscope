// Package settlement は入金・消込モジュールの公開 API。
// 「実際に入金された事実」を債権に適用し、Invoice の入金済み化をトリガする。
// クレカは決済成功＝即入金。口座振替・振込は後日確定するため、銀行入金データの取込経路を持つ。
package settlement

import (
	"context"
	"fmt"
	"log"

	"github.com/kato0373i/subscope/backend/internal/settlement/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// DepositInput は銀行入金データ取込の 1 レコード（取込バッチの入力）。
// 実運用では銀行 CSV/全銀フォーマットを ACL でこの形に正規化する想定。
type DepositInput struct {
	Reference string                  // 入金参照番号（冪等キー）
	Account   shared.BillingAccountID // 振込人の請求先 ID（摘要の顧客コード等から解決。無い場合は名義で照合）
	PayerName string                  // 振込人名義（揺れあり）
	Amount    shared.Money
}

// PayerNameResolver は請求先 ID から登録名義を引く。名義ベースの自動照合に使う（任意）。
type PayerNameResolver func(shared.BillingAccountID) string

type Service struct {
	bus         shared.EventBus
	settlements map[shared.SettlementID]*domain.Settlement
	deposits    map[shared.SettlementID]*domain.BankDeposit
	// outstanding は未消込の請求の投影。InvoiceIssued で積み、消込が進むと減らす。
	outstanding map[shared.InvoiceID]*domain.Candidate
	payerName   PayerNameResolver
	// seen は消込済みの入金トランザクション。PSP の二重通知による二重消込を防ぐ。
	seen map[shared.TransactionID]bool
	// seenRefs は取込済みの入金参照番号。同一入金の二重取込を防ぐ。
	seenRefs map[string]bool
	seq      int
	depSeq   int
}

func NewService(bus shared.EventBus) *Service {
	return NewServiceWithPayerNames(bus, nil)
}

// NewServiceWithPayerNames は名義解決器を注入できるサービスを生成する。
// resolver が nil の場合、名義ベースの照合は行わず請求先 ID ベースで照合する。
func NewServiceWithPayerNames(bus shared.EventBus, resolver PayerNameResolver) *Service {
	s := &Service{
		bus:         bus,
		settlements: make(map[shared.SettlementID]*domain.Settlement),
		deposits:    make(map[shared.SettlementID]*domain.BankDeposit),
		outstanding: make(map[shared.InvoiceID]*domain.Candidate),
		payerName:   resolver,
		seen:        make(map[shared.TransactionID]bool),
		seenRefs:    make(map[string]bool),
	}
	// 未消込請求の投影を作るため発行を購読する。
	bus.Subscribe(events.NameInvoiceIssued, s.onInvoiceIssued)
	// クレカは決済成功＝即入金。口座振替/振込は ImportBankDeposits 経由で後日取り込む。
	bus.Subscribe(events.NamePaymentSucceeded, s.onPaymentSucceeded)
	return s
}

func (s *Service) onInvoiceIssued(_ context.Context, e shared.Event) error {
	ev := e.(events.InvoiceIssued)
	name := ""
	if s.payerName != nil {
		name = s.payerName(ev.BillingAccountID)
	}
	s.outstanding[ev.InvoiceID] = &domain.Candidate{
		Invoice:     ev.InvoiceID,
		Account:     ev.BillingAccountID,
		PayerName:   name,
		Outstanding: ev.Amount,
	}
	return nil
}

func (s *Service) onPaymentSucceeded(ctx context.Context, e shared.Event) error {
	ev := e.(events.PaymentSucceeded)

	// 冪等性：同一の入金（PSP の二重通知）は一度だけ消し込む。TransactionID を自然キーにする。
	if s.seen[ev.TransactionID] {
		log.Printf("[settlement] 重複入金通知を無視 txn=%s（冪等）", ev.TransactionID)
		return nil
	}
	s.seen[ev.TransactionID] = true

	s.seq++
	st := domain.New(shared.SettlementID(fmt.Sprintf("STL-%04d", s.seq)), ev.InvoiceID, ev.Amount)
	// 入金額の全額を債権へ充当する（過消込はドメインが弾く）。
	if err := st.Reconcile(ev.Amount); err != nil {
		return err
	}
	s.settlements[st.ID] = st
	log.Printf("[settlement] 入金を消し込み settlement=%s invoice=%s amount=%s", st.ID, ev.InvoiceID, st.Amount)
	return s.applyToInvoice(ctx, ev.InvoiceID, ev.Amount)
}

// ImportBankDeposits は銀行入金データ取込バッチの入口。
// 各入金を未消込請求へ自動照合（請求先 ID／名義 ＋ 金額）して消し込み、
// 自動照合できなかった入金は UnmatchedDepositDetected を発行して手動消込へ回す。
func (s *Service) ImportBankDeposits(ctx context.Context, inputs []DepositInput) error {
	for _, in := range inputs {
		if in.Reference != "" && s.seenRefs[in.Reference] {
			log.Printf("[settlement] 重複入金を無視 ref=%s（冪等）", in.Reference)
			continue
		}
		if in.Reference != "" {
			s.seenRefs[in.Reference] = true
		}

		s.depSeq++
		dep := domain.NewBankDeposit(
			shared.SettlementID(fmt.Sprintf("DEP-%04d", s.depSeq)),
			in.Reference, in.Account, in.PayerName, in.Amount,
		)
		s.deposits[dep.ID] = dep

		allocs, matched := domain.Match(dep, s.candidates())
		if !matched {
			log.Printf("[settlement] 自動消込できない入金を検出 ref=%s payer=%q amount=%s → 手動消込へ", in.Reference, in.PayerName, in.Amount)
			if err := s.bus.Publish(ctx, events.UnmatchedDepositDetected{
				Reference: in.Reference,
				Account:   in.Account,
				PayerName: in.PayerName,
				Amount:    in.Amount,
			}); err != nil {
				return err
			}
			continue
		}

		for _, a := range allocs {
			if err := dep.Allocate(a.Invoice, a.Amount); err != nil {
				return err
			}
			if err := s.applyToInvoice(ctx, a.Invoice, a.Amount); err != nil {
				return err
			}
		}
	}
	return nil
}

// ReconcileManually はオペレータによる手動消込。自動照合できなかった入金を、
// 担当者が請求を指定して充当する経路（自動／手動ハイブリッドの手動側）。
func (s *Service) ReconcileManually(ctx context.Context, invoice shared.InvoiceID, amount shared.Money) error {
	c, ok := s.outstanding[invoice]
	if !ok {
		return fmt.Errorf("settlement: 未消込の請求が見つかりません invoice=%s", invoice)
	}
	if amount.Currency != c.Outstanding.Currency {
		return domain.ErrCurrencyMismatch
	}
	if amount.Amount > c.Outstanding.Amount {
		return domain.ErrOverApplication
	}
	log.Printf("[settlement] 手動消込 invoice=%s amount=%s", invoice, amount)
	return s.applyToInvoice(ctx, invoice, amount)
}

// applyToInvoice は請求の残額を減らし、全額充当なら InvoicePaid、一部なら InvoicePartiallyPaid を発行する。
func (s *Service) applyToInvoice(ctx context.Context, invoice shared.InvoiceID, amount shared.Money) error {
	c, ok := s.outstanding[invoice]
	if !ok {
		// 投影が無い（既に消込済み、または発行前のクレカ即時入金）。全額消込として扱う。
		return s.bus.Publish(ctx, events.InvoicePaid{InvoiceID: invoice})
	}
	remaining := shared.Money{Amount: c.Outstanding.Amount - amount.Amount, Currency: c.Outstanding.Currency}
	if remaining.Amount <= 0 {
		delete(s.outstanding, invoice)
		log.Printf("[settlement] 請求を全額消込 invoice=%s", invoice)
		return s.bus.Publish(ctx, events.InvoicePaid{InvoiceID: invoice})
	}
	c.Outstanding = remaining
	log.Printf("[settlement] 請求を部分消込 invoice=%s 入金=%s 残=%s", invoice, amount, remaining)
	return s.bus.Publish(ctx, events.InvoicePartiallyPaid{
		InvoiceID:       invoice,
		PaidAmount:      amount,
		RemainingAmount: remaining,
	})
}

// candidates は現在の未消込請求の投影をスライスで返す。
func (s *Service) candidates() []domain.Candidate {
	out := make([]domain.Candidate, 0, len(s.outstanding))
	for _, c := range s.outstanding {
		out = append(out, *c)
	}
	return out
}
