// Package integration_test は中核の縦串フロー
// （契約→請求→回収→決済→消込）を実バス経由で結合テストする。
// 「債権は決済手段を一切参照せず、手段の切替は collection/payment の中だけで起きる」
// という本システムの中核命題を、イベント列で固定するのが狙い。
package integration_test

import (
	"context"
	"sync"
	"testing"

	"github.com/kato0373i/subscope/backend/internal/billing"
	"github.com/kato0373i/subscope/backend/internal/collection"
	"github.com/kato0373i/subscope/backend/internal/contract"
	"github.com/kato0373i/subscope/backend/internal/payment"
	"github.com/kato0373i/subscope/backend/internal/platform/eventbus"
	"github.com/kato0373i/subscope/backend/internal/settlement"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// recorder はバスを流れた全イベントを発生順に記録する。
type recorder struct {
	mu     sync.Mutex
	events []shared.Event
}

func (r *recorder) capture(_ context.Context, e shared.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
	return nil
}

func (r *recorder) count(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if e.EventName() == name {
			n++
		}
	}
	return n
}

// chargeMethods は ChargeRequested で要求された決済手段を発生順に返す。
func (r *recorder) chargeMethods() []shared.PaymentMethodID {
	r.mu.Lock()
	defer r.mu.Unlock()
	var ms []shared.PaymentMethodID
	for _, e := range r.events {
		if c, ok := e.(events.ChargeRequested); ok {
			ms = append(ms, c.PaymentMethodID)
		}
	}
	return ms
}

// 主カード失敗 → 戦略で別カードへ切替 → 成功 → 消込 → 請求書入金済み、
// という回収フロー全体が成立することを固定する。
func TestEndToEnd_MethodFallbackRecovers(t *testing.T) {
	bus := eventbus.NewInMemory()

	// 全イベントを発生順に記録する。バスは同期・深さ優先でディスパッチするため、
	// ネストした発行より先に記録されるよう、サービスより前に購読しておく。
	rec := &recorder{}
	for _, name := range []string{
		events.NameBillingDue,
		events.NameInvoiceIssued,
		events.NameChargeRequested,
		events.NamePaymentFailed,
		events.NamePaymentSucceeded,
		events.NameInvoicePaid,
		events.NameCollectionEscalated,
	} {
		bus.Subscribe(name, rec.capture)
	}

	contracts := contract.NewService(bus)
	_ = billing.NewService(bus)
	_ = collection.NewService(bus)
	_ = payment.NewService(bus)
	_ = settlement.NewService(bus)

	contracts.RegisterContract("CT-1", "MEM-1", "BA-1", shared.JPY(3000))
	if err := contracts.TriggerBilling(context.Background(), "CT-1"); err != nil {
		t.Fatalf("TriggerBilling: %v", err)
	}

	// 請求は一度きり発行され、入金まで到達する。
	if got := rec.count(events.NameInvoiceIssued); got != 1 {
		t.Errorf("InvoiceIssued = %d, want 1", got)
	}
	if got := rec.count(events.NameInvoicePaid); got != 1 {
		t.Errorf("InvoicePaid = %d, want 1", got)
	}

	// 主カードで 1 回失敗し、フォールバックの別カードで成功する。
	if got := rec.count(events.NamePaymentFailed); got != 1 {
		t.Errorf("PaymentFailed = %d, want 1", got)
	}
	if got := rec.count(events.NamePaymentSucceeded); got != 1 {
		t.Errorf("PaymentSucceeded = %d, want 1", got)
	}

	// 手段の切替が collection の戦略どおりに起きている。
	methods := rec.chargeMethods()
	want := []shared.PaymentMethodID{"PM-card-primary", "PM-card-secondary"}
	if len(methods) != len(want) {
		t.Fatalf("ChargeRequested 手段列 = %v, want %v", methods, want)
	}
	for i := range want {
		if methods[i] != want[i] {
			t.Errorf("ChargeRequested[%d] = %q, want %q", i, methods[i], want[i])
		}
	}

	// 回収に成功したのでエスカレーションは起きない。
	if got := rec.count(events.NameCollectionEscalated); got != 0 {
		t.Errorf("CollectionEscalated = %d, want 0", got)
	}
}
