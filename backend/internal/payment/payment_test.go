package payment_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kato0373i/subscope/backend/internal/payment"
	"github.com/kato0373i/subscope/backend/internal/platform/eventbus"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// 副カード（PM-card-secondary）は成功を模擬し、PaymentSucceeded を発行する。
func TestService_ChargeSucceeds(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = payment.NewService(bus)

	var succeeded, failed int
	bus.Subscribe(events.NamePaymentSucceeded, func(context.Context, shared.Event) error { succeeded++; return nil })
	bus.Subscribe(events.NamePaymentFailed, func(context.Context, shared.Event) error { failed++; return nil })

	err := bus.Publish(context.Background(), events.ChargeRequested{
		InvoiceID:       "INV-1",
		PaymentMethodID: "PM-card-secondary",
		Amount:          shared.JPY(3000),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if succeeded != 1 || failed != 0 {
		t.Errorf("succeeded=%d failed=%d, want 1/0", succeeded, failed)
	}
}

// 主カードはゲートウェイ失敗を模擬し、PaymentFailed を発行する。
func TestService_ChargeFails(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = payment.NewService(bus)

	var got *events.PaymentFailed
	bus.Subscribe(events.NamePaymentFailed, func(_ context.Context, e shared.Event) error {
		ev := e.(events.PaymentFailed)
		got = &ev
		return nil
	})

	err := bus.Publish(context.Background(), events.ChargeRequested{
		InvoiceID:       "INV-1",
		PaymentMethodID: "PM-card-primary",
		Amount:          shared.JPY(3000),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if got == nil {
		t.Fatal("PaymentFailed が発行されなかった")
	}
	if got.PaymentMethodID != "PM-card-primary" {
		t.Errorf("PaymentMethodID = %q, want PM-card-primary", got.PaymentMethodID)
	}
	if got.Reason == "" {
		t.Error("FailureReason が空")
	}
}

// 口座振替・払込票は即時確定せず、PaymentPending を発行する（成功/失敗のいずれも出さない）。
func TestService_ChargePending(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = payment.NewService(bus)

	var pending *events.PaymentPending
	var succeeded, failed int
	bus.Subscribe(events.NamePaymentPending, func(_ context.Context, e shared.Event) error {
		ev := e.(events.PaymentPending)
		pending = &ev
		return nil
	})
	bus.Subscribe(events.NamePaymentSucceeded, func(context.Context, shared.Event) error { succeeded++; return nil })
	bus.Subscribe(events.NamePaymentFailed, func(context.Context, shared.Event) error { failed++; return nil })

	err := bus.Publish(context.Background(), events.ChargeRequested{
		InvoiceID:       "INV-1",
		PaymentMethodID: "PM-bank-transfer",
		Amount:          shared.JPY(3000),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if pending == nil {
		t.Fatal("PaymentPending が発行されなかった")
	}
	if pending.PaymentMethodID != "PM-bank-transfer" || pending.Amount != shared.JPY(3000) {
		t.Errorf("PaymentPending = %+v", *pending)
	}
	if succeeded != 0 || failed != 0 {
		t.Errorf("succeeded=%d failed=%d, want 0/0（後日確定待ち）", succeeded, failed)
	}
}

// PSP ゲートウェイ（ACL）を StubGateway に差し替えると、payment の確定結果がそれに従う。
// 実際の決済サービスに接続せず、成功/失敗/pending を手段ごとに宣言的に再現できる。
func TestService_StubGatewayDrivesOutcome(t *testing.T) {
	cases := []struct {
		name                                   string
		configure                              func(*payment.StubGateway)
		method                                 shared.PaymentMethodID
		wantSucceeded, wantPending, wantFailed int
	}{
		{
			name:          "成功シナリオ",
			configure:     func(g *payment.StubGateway) { g.Captures("PM-x") },
			method:        "PM-x",
			wantSucceeded: 1,
		},
		{
			name:       "失敗シナリオ",
			configure:  func(g *payment.StubGateway) { g.Fails("PM-x", "insufficient_funds") },
			method:     "PM-x",
			wantFailed: 1,
		},
		{
			name:        "後日確定シナリオ",
			configure:   func(g *payment.StubGateway) { g.Pends("PM-x") },
			method:      "PM-x",
			wantPending: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bus := eventbus.NewInMemory()
			gw := payment.NewStubGateway()
			tc.configure(gw)
			_ = payment.NewServiceWithGateway(bus, gw)

			var succeeded, pending, failed int
			bus.Subscribe(events.NamePaymentSucceeded, func(context.Context, shared.Event) error { succeeded++; return nil })
			bus.Subscribe(events.NamePaymentPending, func(context.Context, shared.Event) error { pending++; return nil })
			bus.Subscribe(events.NamePaymentFailed, func(context.Context, shared.Event) error { failed++; return nil })

			err := bus.Publish(context.Background(), events.ChargeRequested{
				InvoiceID:       "INV-1",
				PaymentMethodID: tc.method,
				Amount:          shared.JPY(3000),
			})
			if err != nil {
				t.Fatalf("Publish: %v", err)
			}
			if succeeded != tc.wantSucceeded || pending != tc.wantPending || failed != tc.wantFailed {
				t.Errorf("succeeded=%d pending=%d failed=%d, want %d/%d/%d",
					succeeded, pending, failed, tc.wantSucceeded, tc.wantPending, tc.wantFailed)
			}
			// スタブはちょうど 1 回・要求どおりの手段で呼ばれる。
			if calls := gw.Calls(); len(calls) != 1 || calls[0].PaymentMethodID != tc.method {
				t.Errorf("Calls = %+v, want 1 件 method=%s", calls, tc.method)
			}
		})
	}
}

// StubGateway で主カード失敗 → 別カード成功のフォールバックを組み立て、
// collection の手段切替が戦略どおり起きることをスタブの呼び出し記録で検証する。
func TestService_StubGatewayRecordsFallback(t *testing.T) {
	bus := eventbus.NewInMemory()
	gw := payment.NewStubGateway().
		Fails("PM-card-primary", "insufficient_funds")
	// PM-card-secondary は既定（captured）で成功する。
	_ = payment.NewServiceWithGateway(bus, gw)

	var succeeded, failed int
	bus.Subscribe(events.NamePaymentSucceeded, func(context.Context, shared.Event) error { succeeded++; return nil })
	bus.Subscribe(events.NamePaymentFailed, func(context.Context, shared.Event) error { failed++; return nil })

	// 主カード → 失敗、別カード → 成功、を順に流す（collection 抜きで payment 単体を駆動）。
	for _, m := range []shared.PaymentMethodID{"PM-card-primary", "PM-card-secondary"} {
		if err := bus.Publish(context.Background(), events.ChargeRequested{
			InvoiceID:       "INV-1",
			PaymentMethodID: m,
			Amount:          shared.JPY(3000),
		}); err != nil {
			t.Fatalf("Publish(%s): %v", m, err)
		}
	}

	if succeeded != 1 || failed != 1 {
		t.Errorf("succeeded=%d failed=%d, want 1/1", succeeded, failed)
	}
	calls := gw.Calls()
	want := []shared.PaymentMethodID{"PM-card-primary", "PM-card-secondary"}
	if len(calls) != len(want) {
		t.Fatalf("Calls = %d 件, want %d", len(calls), len(want))
	}
	for i, m := range want {
		if calls[i].PaymentMethodID != m {
			t.Errorf("Calls[%d] = %q, want %q", i, calls[i].PaymentMethodID, m)
		}
	}
}

// erroringGateway は PSP の一時エラーを模擬する。
type erroringGateway struct{ err error }

func (g erroringGateway) Charge(context.Context, payment.ChargeInput) (payment.ChargeResult, error) {
	return payment.ChargeResult{}, g.err
}

// PSP 一時エラーで途中終了した場合、冪等キーは消費されず再試行できる。
func TestService_TransientErrorKeepsKeyRetryable(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = payment.NewServiceWithGateway(bus, erroringGateway{err: errors.New("psp timeout")})

	charge := events.ChargeRequested{
		InvoiceID:       "INV-1",
		PaymentMethodID: "PM-x",
		Amount:          shared.JPY(3000),
		IdempotencyKey:  "charge:INV-1:0",
	}
	// 1 回目は PSP エラーで失敗する（キーは記録されない）。
	if err := bus.Publish(context.Background(), charge); err == nil {
		t.Fatal("PSP エラーが伝播しなかった")
	}
	// 2 回目（再試行）は「重複」として握り潰されず、再びゲートウェイへ到達する＝同じエラー。
	if err := bus.Publish(context.Background(), charge); err == nil {
		t.Fatal("再試行が冪等で握り潰された（途中失敗はキーを消費すべきでない）")
	}
}

// 未知の Outcome（未初期化結果など）は成功扱いせず error にする。
func TestService_UnknownOutcomeIsError(t *testing.T) {
	bus := eventbus.NewInMemory()
	// err=nil・ChargeResult{} のゼロ値（OutcomeUnknown）を返すスタブ。
	_ = payment.NewServiceWithGateway(bus, erroringGateway{err: nil})

	var succeeded int
	bus.Subscribe(events.NamePaymentSucceeded, func(context.Context, shared.Event) error { succeeded++; return nil })

	err := bus.Publish(context.Background(), events.ChargeRequested{
		InvoiceID:       "INV-1",
		PaymentMethodID: "PM-x",
		Amount:          shared.JPY(3000),
	})
	if err == nil {
		t.Fatal("未知の Outcome が error にならなかった")
	}
	if succeeded != 0 {
		t.Errorf("PaymentSucceeded = %d, want 0（未課金を成功扱いしない）", succeeded)
	}
}

// 同一の課金要求（冪等キー）が再送されても、決済は一度だけ実行される（二重決済しない）。
func TestService_ChargeIsIdempotent(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = payment.NewService(bus)

	var succeeded int
	bus.Subscribe(events.NamePaymentSucceeded, func(context.Context, shared.Event) error { succeeded++; return nil })

	charge := events.ChargeRequested{
		InvoiceID:       "INV-1",
		PaymentMethodID: "PM-card-secondary",
		Amount:          shared.JPY(3000),
		IdempotencyKey:  "charge:INV-1:0",
	}
	// PSP webhook 二重通知を模擬し、同じ課金要求を 2 回発行する。
	if err := bus.Publish(context.Background(), charge); err != nil {
		t.Fatalf("Publish#1: %v", err)
	}
	if err := bus.Publish(context.Background(), charge); err != nil {
		t.Fatalf("Publish#2: %v", err)
	}

	if succeeded != 1 {
		t.Errorf("PaymentSucceeded = %d, want 1（冪等キーで二重決済を抑止）", succeeded)
	}
}
