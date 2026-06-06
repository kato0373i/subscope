package domain

import (
	"testing"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

func TestCreditCard_IsUsable(t *testing.T) {
	pm := NewCreditCard("PM-001", "BA-001", "tok_001", 1)
	if !pm.IsUsable() {
		t.Fatal("登録直後のクレカは使用可能でなければならない")
	}
	pm.Expire()
	if pm.IsUsable() {
		t.Fatal("失効後は使用不可でなければならない")
	}
}

func TestBankAccount_RegistrationStateMachine(t *testing.T) {
	pm := NewBankAccount("PM-002", shared.BillingAccountID("BA-001"), "tok_bank", 2)

	// 登録直後は使用不可
	if pm.IsUsable() {
		t.Fatal("登録直後の口座振替は使用不可でなければならない")
	}
	if *pm.RegistrationStatus() != RegStatusPending {
		t.Fatal("初期状態は pending でなければならない")
	}

	// 審査開始
	if err := pm.StartReview(); err != nil {
		t.Fatalf("StartReview: %v", err)
	}
	if *pm.RegistrationStatus() != RegStatusReviewing {
		t.Fatal("StartReview 後は reviewing でなければならない")
	}
	if pm.IsUsable() {
		t.Fatal("審査中は使用不可でなければならない")
	}

	// 登録完了
	if err := pm.CompleteRegistration(); err != nil {
		t.Fatalf("CompleteRegistration: %v", err)
	}
	if *pm.RegistrationStatus() != RegStatusCompleted {
		t.Fatal("CompleteRegistration 後は completed でなければならない")
	}
	if !pm.IsUsable() {
		t.Fatal("登録完了後は使用可能でなければならない")
	}

	// 二重完了はエラー
	if err := pm.CompleteRegistration(); err != ErrAlreadyCompleted {
		t.Fatalf("二重完了は ErrAlreadyCompleted を返すべき、got: %v", err)
	}
}

func TestBankAccount_Rejection(t *testing.T) {
	pm := NewBankAccount("PM-003", shared.BillingAccountID("BA-001"), "tok_bank", 3)
	if err := pm.RejectRegistration(); err != nil {
		t.Fatalf("RejectRegistration: %v", err)
	}
	if *pm.RegistrationStatus() != RegStatusRejected {
		t.Fatal("否認後は rejected でなければならない")
	}
	if pm.IsUsable() {
		t.Fatal("否認後は使用不可でなければならない")
	}
}

func TestCreditCard_StartReview_Error(t *testing.T) {
	pm := NewCreditCard("PM-004", "BA-001", "tok", 1)
	if err := pm.StartReview(); err != ErrNotBankAccount {
		t.Fatalf("クレカへの StartReview は ErrNotBankAccount を返すべき、got: %v", err)
	}
}
