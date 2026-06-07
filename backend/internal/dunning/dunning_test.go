package dunning_test

import (
	"context"
	"testing"

	"github.com/kato0373i/subscope/backend/internal/dunning"
	"github.com/kato0373i/subscope/backend/internal/platform/eventbus"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// エスカレーションで督促を起票し、D+0（email）ステップを発火する。請求先 ID も載る。
func TestService_EscalationStartsCampaignAndFirstStep(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = dunning.NewService(bus)

	var step *events.DunningStepTriggered
	bus.Subscribe(events.NameDunningStepTriggered, func(_ context.Context, e shared.Event) error {
		ev := e.(events.DunningStepTriggered)
		step = &ev
		return nil
	})

	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})
	mustPublish(t, bus, events.CollectionEscalated{CaseID: "CASE-1", InvoiceID: "INV-1", Amount: shared.JPY(3000)})

	if step == nil {
		t.Fatal("DunningStepTriggered が発行されなかった")
	}
	if step.Channel != "email" || step.StepNumber != 1 {
		t.Errorf("初回ステップ = (%s,%d), want (email,1)", step.Channel, step.StepNumber)
	}
	if step.Account != "BA-1" {
		t.Errorf("Account = %q, want BA-1", step.Account)
	}
}

// 同一請求への重複した失敗・エスカレーションでは多重起票しない（最初の 1 回だけ発火）。
func TestService_DoesNotDuplicateCampaign(t *testing.T) {
	bus := eventbus.NewInMemory()
	_ = dunning.NewService(bus)

	var steps int
	bus.Subscribe(events.NameDunningStepTriggered, func(context.Context, shared.Event) error { steps++; return nil })

	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})
	mustPublish(t, bus, events.PaymentFailed{InvoiceID: "INV-1", PaymentMethodID: "PM-card-primary", Reason: "x"})
	mustPublish(t, bus, events.PaymentFailed{InvoiceID: "INV-1", PaymentMethodID: "PM-card-secondary", Reason: "x"})
	mustPublish(t, bus, events.CollectionEscalated{CaseID: "CASE-1", InvoiceID: "INV-1", Amount: shared.JPY(3000)})

	if steps != 1 {
		t.Errorf("DunningStepTriggered = %d, want 1（多重起票しない）", steps)
	}
}

// AdvanceCampaigns で次の督促段階に進む。シーケンスを使い切ると発火が止まる。
func TestService_AdvanceCampaignsRunsSequence(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := dunning.NewService(bus)

	var channels []string
	bus.Subscribe(events.NameDunningStepTriggered, func(_ context.Context, e shared.Event) error {
		channels = append(channels, e.(events.DunningStepTriggered).Channel)
		return nil
	})

	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})
	mustPublish(t, bus, events.CollectionEscalated{CaseID: "CASE-1", InvoiceID: "INV-1", Amount: shared.JPY(3000)})
	// 起票で email、以降 2 回の Advance で sms・letter。
	mustAdvance(t, s, bus)
	mustAdvance(t, s, bus)
	mustAdvance(t, s, bus) // シーケンス尽きた後は何も起きない

	want := []string{"email", "sms", "letter"}
	if len(channels) != len(want) {
		t.Fatalf("発火チャネル列 = %v, want %v", channels, want)
	}
	for i := range want {
		if channels[i] != want[i] {
			t.Errorf("step %d = %q, want %q", i, channels[i], want[i])
		}
	}
}

// 入金（InvoicePaid）で督促は止まり、以降 Advance しても発火しない。
func TestService_InvoicePaidStopsDunning(t *testing.T) {
	bus := eventbus.NewInMemory()
	s := dunning.NewService(bus)

	var steps int
	bus.Subscribe(events.NameDunningStepTriggered, func(context.Context, shared.Event) error { steps++; return nil })

	mustPublish(t, bus, events.InvoiceIssued{InvoiceID: "INV-1", BillingAccountID: "BA-1", Amount: shared.JPY(3000)})
	mustPublish(t, bus, events.CollectionEscalated{CaseID: "CASE-1", InvoiceID: "INV-1", Amount: shared.JPY(3000)})
	mustPublish(t, bus, events.InvoicePaid{InvoiceID: "INV-1"})
	mustAdvance(t, s, bus)

	if steps != 1 {
		t.Errorf("DunningStepTriggered = %d, want 1（入金後は止まる）", steps)
	}
}

func mustPublish(t *testing.T, bus shared.EventBus, e shared.Event) {
	t.Helper()
	if err := bus.Publish(context.Background(), e); err != nil {
		t.Fatalf("Publish(%s): %v", e.EventName(), err)
	}
}

func mustAdvance(t *testing.T, s *dunning.Service, _ shared.EventBus) {
	t.Helper()
	if err := s.AdvanceCampaigns(context.Background()); err != nil {
		t.Fatalf("AdvanceCampaigns: %v", err)
	}
}
