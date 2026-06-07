package domain

import "testing"

func TestNew_StartsQueued(t *testing.T) {
	n := New("NTF-1", "INV-1", "BA-1", "email", "email:BA-1")
	if n.Status != StatusQueued {
		t.Errorf("Status = %q, want %q", n.Status, StatusQueued)
	}
}

func TestMarkSent_FromQueued(t *testing.T) {
	n := New("NTF-1", "INV-1", "BA-1", "email", "email:BA-1")
	if !n.MarkSent() {
		t.Fatal("MarkSent = false, want true")
	}
	if n.Status != StatusSent {
		t.Errorf("Status = %q, want %q", n.Status, StatusSent)
	}
	// 送信済みからは再送信扱いにできない。
	if n.MarkSent() {
		t.Error("送信済みからの MarkSent は false であるべき")
	}
}

func TestMarkFailed_FromQueued(t *testing.T) {
	n := New("NTF-1", "INV-1", "BA-1", "sms", "sms:BA-1")
	if !n.MarkFailed() {
		t.Fatal("MarkFailed = false, want true")
	}
	if n.Status != StatusFailed {
		t.Errorf("Status = %q, want %q", n.Status, StatusFailed)
	}
}
