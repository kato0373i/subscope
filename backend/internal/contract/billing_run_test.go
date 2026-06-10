package contract_test

import (
	"context"
	"testing"
	"time"

	"github.com/kato0373i/subscope/backend/internal/billing"
	"github.com/kato0373i/subscope/backend/internal/collection"
	"github.com/kato0373i/subscope/backend/internal/contract"
	"github.com/kato0373i/subscope/backend/internal/platform/eventbus"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// Billing Run は課金対象の契約だけを抽出し、契約ごとに BillingDue を発行する。
// BillingDue は金額と請求先だけを載せ、決済手段への参照を持たない（債権≠決済手段）。
func TestService_RunBillingExtractsDueContracts(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := contract.NewService(bus)

	var dues []events.BillingDue
	bus.Subscribe(events.NameBillingDue, func(_ context.Context, e shared.Event) error {
		dues = append(dues, e.(events.BillingDue))
		return nil
	})

	s.RegisterContract("CT-1", "MEM-1", "BA-1", shared.JPY(3000))
	s.RegisterContract("CT-2", "MEM-2", "BA-2", shared.JPY(5000))
	s.RegisterTrial("CT-3", "MEM-3", "BA-3", shared.JPY(8000), 14) // trialing は課金対象外

	asOf := time.Now().AddDate(1, 0, 0) // 確実に請求サイクル到来済みにする
	res, err := s.RunBilling(context.Background(), asOf, false)
	if err != nil {
		t.Fatalf("RunBilling: %v", err)
	}

	if len(res.Items) != 2 {
		t.Fatalf("起票件数 = %d, want 2 (%+v)", len(res.Items), res.Items)
	}
	if len(dues) != 2 {
		t.Fatalf("BillingDue 発行数 = %d, want 2", len(dues))
	}
	// 発行されたイベントは金額・請求先を載せる。型に決済手段フィールドが無い＝構造的に疎結合。
	byContract := map[shared.ContractID]events.BillingDue{}
	for _, d := range dues {
		byContract[d.ContractID] = d
	}
	if byContract["CT-1"].Amount != shared.JPY(3000) {
		t.Errorf("CT-1 Amount = %v, want 3000", byContract["CT-1"].Amount)
	}
	if byContract["CT-1"].BillingAccountID != "BA-1" {
		t.Errorf("CT-1 BillingAccountID = %q, want BA-1", byContract["CT-1"].BillingAccountID)
	}
	if byContract["CT-1"].Period == "" {
		t.Error("CT-1 Period が空（請求スケジュールから導出されていない）")
	}
}

// ドライランは抽出結果だけを返し、イベント発行も次回請求日の前進も行わない。
// 続く実行は同じ期間を起票できる（ドライランは副作用を残さない）。
func TestService_RunBillingDryRunNoSideEffects(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := contract.NewService(bus)

	var dues int
	bus.Subscribe(events.NameBillingDue, func(context.Context, shared.Event) error { dues++; return nil })

	s.RegisterContract("CT-1", "MEM-1", "BA-1", shared.JPY(3000))
	asOf := time.Now().AddDate(1, 0, 0)

	preview, err := s.RunBilling(context.Background(), asOf, true)
	if err != nil {
		t.Fatalf("RunBilling(dry): %v", err)
	}
	if !preview.DryRun || len(preview.Items) != 1 {
		t.Fatalf("ドライラン結果 = %+v, want DryRun=true Items=1", preview)
	}
	if dues != 0 {
		t.Fatalf("ドライランで BillingDue が %d 件発行された, want 0", dues)
	}

	// ドライラン後でも実行すれば同じ期間を起票できる（前進していない証拠）。
	real, err := s.RunBilling(context.Background(), asOf, false)
	if err != nil {
		t.Fatalf("RunBilling: %v", err)
	}
	if len(real.Items) != 1 || dues != 1 {
		t.Fatalf("実行結果 Items=%d dues=%d, want 1/1", len(real.Items), dues)
	}
}

// 同一期間の二重起票を防ぐ。次回請求日が前進するため、同じ asOf の再実行では対象から外れる。
func TestService_RunBillingDoesNotRebillSamePeriod(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := contract.NewService(bus)

	var dues int
	bus.Subscribe(events.NameBillingDue, func(context.Context, shared.Event) error { dues++; return nil })

	s.RegisterContract("CT-1", "MEM-1", "BA-1", shared.JPY(3000))
	asOf := time.Now().AddDate(0, 1, 0) // ちょうど 1 期間ぶんだけ到来させる

	if _, err := s.RunBilling(context.Background(), asOf, false); err != nil {
		t.Fatalf("RunBilling 1: %v", err)
	}
	res2, err := s.RunBilling(context.Background(), asOf, false)
	if err != nil {
		t.Fatalf("RunBilling 2: %v", err)
	}
	if len(res2.Items) != 0 {
		t.Errorf("再実行の起票件数 = %d, want 0", len(res2.Items))
	}
	if dues != 1 {
		t.Errorf("BillingDue 発行総数 = %d, want 1（二重起票していない）", dues)
	}
}

// 債権↔決済手段の疎結合の実証：Billing Run が起こした請求の決済手段は、後段 collection が
// 請求先ごとに遅延束縛する。請求先のデフォルト手段を差し替えると、新たな自動課金は新しい手段で
// 行われる一方、請求（Invoice）側は手段を一切知らないまま不変に保たれる＝手段の付け替えが可能。
func TestService_AutoBillingPaymentMethodIsSwappable(t *testing.T) {
	bus := eventbus.NewInMemory()

	// 請求先ごとの「現在のデフォルト決済手段」。運用側の差し替えを模す可変ステート。
	defaultMethod := map[shared.BillingAccountID]shared.PaymentMethodID{}
	resolver := func(ba shared.BillingAccountID) collection.Strategy {
		strat := collection.DefaultStrategy()
		strat.MethodFallback = []shared.PaymentMethodID{defaultMethod[ba]}
		return strat
	}

	contracts := contract.NewService(bus)
	invoices := billing.NewService(bus)                  // BillingDue → Invoice（決済手段なし）
	_ = collection.NewServiceWithStrategy(bus, resolver) // InvoiceIssued → ChargeRequested（手段を遅延束縛）

	var charges []events.ChargeRequested
	bus.Subscribe(events.NameChargeRequested, func(_ context.Context, e shared.Event) error {
		charges = append(charges, e.(events.ChargeRequested))
		return nil
	})

	// 同一契約・同一請求先を、デフォルト手段を途中で付け替えながら 2 サイクル自動課金する。
	contracts.RegisterContract("CT-1", "MEM-1", "BA-1", shared.JPY(3000))

	// 1 サイクル目: デフォルトは PM-card-A。
	defaultMethod["BA-1"] = "PM-card-A"
	if _, err := contracts.RunBilling(context.Background(), time.Now().AddDate(0, 1, 0), false); err != nil {
		t.Fatalf("RunBilling 1: %v", err)
	}

	// 請求先のデフォルト手段を PM-card-B へ「付け替え」てから次サイクルを自動課金。
	defaultMethod["BA-1"] = "PM-card-B"
	if _, err := contracts.RunBilling(context.Background(), time.Now().AddDate(0, 2, 0), false); err != nil {
		t.Fatalf("RunBilling 2: %v", err)
	}

	if len(charges) != 2 {
		t.Fatalf("ChargeRequested 数 = %d, want 2 (%+v)", len(charges), charges)
	}
	// 課金は付け替え後のデフォルト手段に追従する（次サイクルは新しい手段で課金される）。
	if charges[0].PaymentMethodID != "PM-card-A" {
		t.Errorf("1 サイクル目の手段 = %q, want PM-card-A", charges[0].PaymentMethodID)
	}
	if charges[1].PaymentMethodID != "PM-card-B" {
		t.Errorf("2 サイクル目の手段 = %q, want PM-card-B（付け替えが反映されていない）", charges[1].PaymentMethodID)
	}

	// 請求（Invoice）側は 2 件とも発行済みだが、決済手段は一切保持しない（型に存在しない）。
	// InvoiceView に決済手段フィールドが無いこと自体が、債権が手段から独立している証拠。
	if got := len(invoices.ListInvoices()); got != 2 {
		t.Errorf("発行済み Invoice = %d, want 2", got)
	}
}
