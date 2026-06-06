package domain

import (
	"errors"
	"fmt"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

type Status string

const (
	StatusRequested Status = "requested"
	StatusCaptured  Status = "captured"
	StatusFailed    Status = "failed"
	StatusPending   Status = "pending" // 口座振替・払込票は結果が後日確定する
)

var (
	// ErrNotRequested は requested 以外からの確定/失敗/保留を弾く。
	ErrNotRequested = errors.New("この遷移は requested 状態からのみ可能です")
	// ErrNotPending は pending 以外からの後日確定を弾く。
	ErrNotPending = errors.New("後日確定は pending 状態からのみ可能です")
)

// Transaction は 1 回の決済試行。invoice_id と payment_method_id がここで初めて出会う。
// 1 試行は 1 手段・1 金額に固定し、手段を変える場合は新しい Transaction を作る
// （Invoice / PaymentMethod / Amount に setter を置かないことで構造的に保証する）。
//
// 状態機械:
//
//	requested ─┬─► captured        （クレカ等：即時確定）
//	           ├─► pending ─► captured（口座振替・払込票：後日 settlement が確定）
//	           └─► failed          （残高不足・限度額・口座エラー…）
type Transaction struct {
	ID            shared.TransactionID
	Invoice       shared.InvoiceID
	PaymentMethod shared.PaymentMethodID
	Amount        shared.Money
	Status        Status
	FailureReason string
}

func NewTransaction(id shared.TransactionID, invoice shared.InvoiceID, method shared.PaymentMethodID, amount shared.Money) *Transaction {
	return &Transaction{
		ID:            id,
		Invoice:       invoice,
		PaymentMethod: method,
		Amount:        amount,
		Status:        StatusRequested,
	}
}

// MarkCaptured は即時確定（requested → captured）。クレカのように同期で確定する手段で使う。
func (t *Transaction) MarkCaptured() error {
	if t.Status != StatusRequested {
		return fmt.Errorf("%w: 現在=%s", ErrNotRequested, t.Status)
	}
	t.Status = StatusCaptured
	return nil
}

// MarkPending は後日確定待ちへ（requested → pending）。口座振替・払込票で使う。
// 確定の事実は settlement が入金を取り込んだ時点で Confirm により反映される。
func (t *Transaction) MarkPending() error {
	if t.Status != StatusRequested {
		return fmt.Errorf("%w: 現在=%s", ErrNotRequested, t.Status)
	}
	t.Status = StatusPending
	return nil
}

// MarkFailed は失敗（requested → failed）。理由を残し、collection が手段切替を判断する。
func (t *Transaction) MarkFailed(reason string) error {
	if t.Status != StatusRequested {
		return fmt.Errorf("%w: 現在=%s", ErrNotRequested, t.Status)
	}
	t.Status = StatusFailed
	t.FailureReason = reason
	return nil
}

// Confirm は後日入金の確定（pending → captured）。settlement が銀行入金データを
// 取り込んだ時点で呼ばれる想定。pending 以外からは遷移できない。
func (t *Transaction) Confirm() error {
	if t.Status != StatusPending {
		return fmt.Errorf("%w: 現在=%s", ErrNotPending, t.Status)
	}
	t.Status = StatusCaptured
	return nil
}

// IsPending は後日確定待ちかどうか。
func (t *Transaction) IsPending() bool { return t.Status == StatusPending }
