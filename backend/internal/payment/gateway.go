package payment

import (
	"context"
	"strings"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

// Outcome は PSP がこの試行をどう確定したか。確定タイミング（即時/後日）の差は
// この境界で吸収し、payment の本体は手段ごとの PSP 差異を知らずに済む。
type Outcome int

const (
	OutcomeCaptured Outcome = iota // 即時確定（クレカ等）
	OutcomePending                 // 後日確定（口座振替・払込票）
	OutcomeFailed                  // 失敗（残高不足・口座エラー等）
)

// ChargeInput は PSP への課金依頼。決済手段の実体（PSP トークン）は本来この層で
// 解決するが、デモでは PaymentMethodID の命名から手段種別を推定する。
type ChargeInput struct {
	TransactionID   shared.TransactionID
	InvoiceID       shared.InvoiceID
	PaymentMethodID shared.PaymentMethodID
	Amount          shared.Money
}

// ChargeResult は PSP の応答。OutcomeFailed のときのみ Reason を設定する。
type ChargeResult struct {
	Outcome Outcome
	Reason  string
}

// Gateway は PSP（Stripe / GMO / SB ペイメント等）への腐敗防止層（ACL）。
// ゲートウェイ差異・手段ごとの確定タイミングをこの境界に閉じ込め、
// payment の Service はベンダ非依存のまま保つ。本番では PSP ごとの実装に差し替える。
type Gateway interface {
	Charge(ctx context.Context, in ChargeInput) (ChargeResult, error)
}

// MockGateway はデモ・テスト用の PSP スタブ。
// 手段種別を ID 命名から推定し、クレカ=即時確定、口座振替・払込票=後日確定（pending）を模擬する。
// 主カード（PM-card-primary）は残高不足を模擬し、回収戦略のフォールバックを発火させる。
type MockGateway struct{}

func (MockGateway) Charge(_ context.Context, in ChargeInput) (ChargeResult, error) {
	switch {
	case in.PaymentMethodID == "PM-card-primary":
		return ChargeResult{Outcome: OutcomeFailed, Reason: "insufficient_funds"}, nil
	case isDeferred(in.PaymentMethodID):
		return ChargeResult{Outcome: OutcomePending}, nil
	default:
		return ChargeResult{Outcome: OutcomeCaptured}, nil
	}
}

// isDeferred は後日確定する手段（口座振替・払込票・振込）かどうかを ID 命名から推定する。
// デモ専用のヒューリスティクス。実手段への接続は payment_method / collection 側で行う。
func isDeferred(m shared.PaymentMethodID) bool {
	id := string(m)
	return strings.Contains(id, "bank") || strings.Contains(id, "slip") || strings.Contains(id, "transfer")
}
