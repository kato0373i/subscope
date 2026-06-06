package payment

import (
	"context"
	"strings"
	"sync"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

// Outcome は PSP がこの試行をどう確定したか。確定タイミング（即時/後日）の差は
// この境界で吸収し、payment の本体は手段ごとの PSP 差異を知らずに済む。
//
// ゼロ値は OutcomeUnknown（無効値）。未初期化の ChargeResult{} を成功と取り違えないため、
// 呼び出し側は未知の Outcome を error として扱う。
type Outcome int

const (
	OutcomeUnknown  Outcome = iota // 無効値（ゼロ値）。未初期化結果を成功扱いしないための番兵
	OutcomeCaptured                // 即時確定（クレカ等）
	OutcomePending                 // 後日確定（口座振替・払込票）
	OutcomeFailed                  // 失敗（残高不足・口座エラー等）
)

// ChargeInput は PSP への課金依頼。決済手段の実体（PSP トークン）は本来この層で
// 解決するが、デモでは PaymentMethodID の命名から手段種別を推定する。
//
// IdempotencyKey は ChargeRequested から引き継ぎ、Gateway 実装が PSP の冪等 API に
// そのまま渡せるようにする。プロセス内の seen 重複排除はプロセス再起動・多重配信に弱いため、
// 最終的な二重課金防止は PSP 側の冪等キーで担保する。
type ChargeInput struct {
	TransactionID   shared.TransactionID
	InvoiceID       shared.InvoiceID
	PaymentMethodID shared.PaymentMethodID
	Amount          shared.Money
	IdempotencyKey  shared.IdempotencyKey
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

// StubGateway は PoC・テスト用の設定可能な PSP スタブ。
// 実際の決済サービスには接続せず、決済手段ごと（または既定）の結果を宣言的に決められる。
// 「主カードは残高不足、口座振替は pending、それ以外は成功」のような任意のシナリオを、
// ID 命名に依存せず明示的に組み立ててテストできる。
//
// 呼び出しは Calls に発生順で記録されるため、リトライ・手段フォールバックが戦略どおりに
// 起きたか（何回・どの手段で課金したか）も検証できる。
//
//	gw := payment.NewStubGateway().
//		Fails("PM-card-primary", "insufficient_funds").
//		Pends("PM-bank-transfer")
//	svc := payment.NewServiceWithGateway(bus, gw)
//
// 既定（手段未指定）の結果は OutcomeCaptured。Default で変更できる。
type StubGateway struct {
	mu       sync.Mutex
	byMethod map[shared.PaymentMethodID]ChargeResult
	fallback ChargeResult
	calls    []ChargeInput
}

// NewStubGateway は既定で「常に成功（captured）」を返すスタブを作る。
func NewStubGateway() *StubGateway {
	return &StubGateway{
		byMethod: make(map[shared.PaymentMethodID]ChargeResult),
		fallback: ChargeResult{Outcome: OutcomeCaptured},
	}
}

// Returns は特定の決済手段に対する固定結果を登録する（チェーン可能）。
func (g *StubGateway) Returns(method shared.PaymentMethodID, res ChargeResult) *StubGateway {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.byMethod[method] = res
	return g
}

// Captures は指定手段を即時成功（captured）にする。
func (g *StubGateway) Captures(method shared.PaymentMethodID) *StubGateway {
	return g.Returns(method, ChargeResult{Outcome: OutcomeCaptured})
}

// Pends は指定手段を後日確定待ち（pending）にする。
func (g *StubGateway) Pends(method shared.PaymentMethodID) *StubGateway {
	return g.Returns(method, ChargeResult{Outcome: OutcomePending})
}

// Fails は指定手段を失敗（failed）にし、理由を添える。
func (g *StubGateway) Fails(method shared.PaymentMethodID, reason string) *StubGateway {
	return g.Returns(method, ChargeResult{Outcome: OutcomeFailed, Reason: reason})
}

// Default は手段未登録時に返す既定結果を差し替える。
func (g *StubGateway) Default(res ChargeResult) *StubGateway {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.fallback = res
	return g
}

func (g *StubGateway) Charge(_ context.Context, in ChargeInput) (ChargeResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls = append(g.calls, in)
	if res, ok := g.byMethod[in.PaymentMethodID]; ok {
		return res, nil
	}
	return g.fallback, nil
}

// Calls は記録された課金依頼を発生順のコピーで返す（試行回数・手段の検証用）。
func (g *StubGateway) Calls() []ChargeInput {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]ChargeInput(nil), g.calls...)
}
