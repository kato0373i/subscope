package domain

import "testing"

func TestNewCampaign_StartsActiveWithDefaultSequence(t *testing.T) {
	c := NewCampaign("DUN-1", "INV-1", "BA-1", nil)
	if c.Status != StatusActive {
		t.Errorf("Status = %q, want %q", c.Status, StatusActive)
	}
}

// 既定シーケンス（email → sms → letter）を順に発火し、最後で completed になる。
func TestTriggerNext_FollowsSequenceThenCompletes(t *testing.T) {
	c := NewCampaign("DUN-1", "INV-1", "BA-1", nil)
	wantChannels := []Channel{ChannelEmail, ChannelSMS, ChannelLetter}

	for i, want := range wantChannels {
		step, num, ok := c.TriggerNext()
		if !ok {
			t.Fatalf("step %d で発火できなかった", i)
		}
		if step.Channel != want {
			t.Errorf("step %d: channel = %q, want %q", i, step.Channel, want)
		}
		if num != i+1 {
			t.Errorf("step %d: num = %d, want %d", i, num, i+1)
		}
	}
	if c.Status != StatusCompleted {
		t.Errorf("全ステップ後の Status = %q, want %q", c.Status, StatusCompleted)
	}
	if _, _, ok := c.TriggerNext(); ok {
		t.Error("完了後の TriggerNext は false であるべき")
	}
}

// 入金で解決すると以降のステップは発火しない。
func TestResolve_StopsFurtherSteps(t *testing.T) {
	c := NewCampaign("DUN-1", "INV-1", "BA-1", nil)
	if _, _, ok := c.TriggerNext(); !ok {
		t.Fatal("1 ステップ目の発火に失敗")
	}
	if !c.Resolve() {
		t.Fatal("Resolve = false, want true")
	}
	if c.Status != StatusResolved {
		t.Errorf("Status = %q, want %q", c.Status, StatusResolved)
	}
	if _, _, ok := c.TriggerNext(); ok {
		t.Error("解決後の TriggerNext は false であるべき")
	}
}

// 解決は進行中からのみ。完了済みからは解決しない。
func TestResolve_NoOpAfterCompleted(t *testing.T) {
	c := NewCampaign("DUN-1", "INV-1", "BA-1", []Step{{OffsetDays: 0, Channel: ChannelEmail}})
	c.TriggerNext() // 1 ステップで completed
	if c.Resolve() {
		t.Error("完了後の Resolve は false であるべき")
	}
	if c.Status != StatusCompleted {
		t.Errorf("Status = %q, want %q", c.Status, StatusCompleted)
	}
}
