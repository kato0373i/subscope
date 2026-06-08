package domain

import "testing"

func TestDelivery_SuccessFromPending(t *testing.T) {
	d := NewDelivery("WD-1", "WE-1", "billing.InvoiceIssued", 3)
	if !d.RecordSuccess() {
		t.Fatal("pending からの成功は受理されるべき")
	}
	if d.Status != StatusDelivered {
		t.Errorf("Status = %q, want delivered", d.Status)
	}
	if d.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", d.Attempts)
	}
	if d.RecordSuccess() {
		t.Error("delivered からの再成功は弾かれるべき")
	}
}

func TestDelivery_RetriesUntilMax(t *testing.T) {
	d := NewDelivery("WD-1", "WE-1", "billing.InvoiceIssued", 3)

	d.RecordFailure() // 1
	if d.Status != StatusPending || !d.CanRetry() {
		t.Fatalf("1 回失敗後は pending かつ再試行可: status=%q canRetry=%v", d.Status, d.CanRetry())
	}
	d.RecordFailure() // 2
	d.RecordFailure() // 3 → 上限
	if d.Status != StatusFailed {
		t.Errorf("Status = %q, want failed", d.Status)
	}
	if d.CanRetry() {
		t.Error("上限到達後は再試行不可であるべき")
	}
}

func TestNewDelivery_MinAttempts(t *testing.T) {
	d := NewDelivery("WD-1", "WE-1", "x", 0)
	d.RecordFailure()
	if d.Status != StatusFailed {
		t.Errorf("maxAttempts<1 は 1 に補正され、1 回失敗で failed: got %q", d.Status)
	}
}
