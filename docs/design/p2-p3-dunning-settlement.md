# P2 / P3 設計 + 実装計画: 督促・入金消込

> ロードマップ第2・第3フェーズ。日次オペの中核である **督促（dunning）** と
> **入金・消込（settlement）** を画面化する。
> P1（顧客360）と違い、両モジュールは**コマンドはあるが読み取り API が無い**ため、
> まず読み取り Service を新設してから画面を作る二段構え。

## 0. 設計判断: CQRS Read Model か、単純 List か

**結論: 独立 Read Model ストアは作らない。各モジュールの Service に `ListX()` を生やす。**

理由: 両 Service は既にイベント購読で**投影 map を内部保持**している。
- `dunning.Service.campaigns map[InvoiceID]*Campaign`
- `settlement.Service.settlements map[SettlementID]*Settlement` ＋ `outstanding map[InvoiceID]*Candidate`

これらを読み出す公開メソッドを足せば一覧は得られる。別ストア・別購読を立てるのは
現段階では過剰（metrics のような跨ぎ集計とは違い、各モジュール内で閉じた読み取り）。
将来クエリが重くなったら専用 Read Model に切り出す（その時の判断材料として本節を残す）。

---

# P2: 督促（dunning）

## P2.1 現状

| 区分 | 公開 API | 状態 |
|---|---|---|
| コマンド | `AdvanceCampaigns(ctx)` | ✅ 全キャンペーンを1ステップ進める |
| 読み取り | — | ❌ 無い |

`Campaign`（domain）は `{ID, Invoice, Account, Status, steps, triggered}` を持つ。
`steps`/`triggered` は非公開。Status は `active` / `resolved`（入金で停止）/ `completed`（全段階実施）。
既定シーケンスは D+0 email → D+3 sms → D+7 letter。

## P2.2 バックエンド設計

### domain アクセサ追加（`campaign.go`）
`triggered`/`steps` が非公開なため、進捗を読むアクセサを1つ足す（純粋・副作用なし）:

```go
// Progress は実施済みステップ数と全ステップ数を返す（読み取り用）。
func (c *Campaign) Progress() (triggered, total int) {
	return c.triggered, len(c.steps)
}

// NextChannel は次に発火するチャネルを返す。
// 進行中(active)でない（入金解決・全実施済み）場合は ""。
// 解決済みキャンペーンが「まだ次の督促がある」ように見えるのを防ぐ。
func (c *Campaign) NextChannel() Channel {
	if c.Status != StatusActive || c.triggered >= len(c.steps) {
		return ""
	}
	return c.steps[c.triggered].Channel
}
```

### 公開 Service に読み取り追加（`dunning.go`）

```go
// CampaignView は督促キャンペーンの外向き読み取りビュー。
type CampaignView struct {
	CampaignID     shared.DunningCampaignID
	InvoiceID      shared.InvoiceID
	Account        shared.BillingAccountID
	Status         string // active / resolved / completed
	StepsTriggered int
	StepsTotal     int
	NextChannel    string // 次段階のチャネル（完了なら ""）
}

// ListCampaigns は進行中・終了済みを含む全督促キャンペーンを返す。
func (s *Service) ListCampaigns() []CampaignView {
	out := make([]CampaignView, 0, len(s.campaigns))
	for _, c := range s.campaigns {
		t, total := c.Progress()
		out = append(out, CampaignView{
			CampaignID: c.ID, InvoiceID: c.Invoice, Account: c.Account,
			Status: string(c.Status), StepsTriggered: t, StepsTotal: total,
			NextChannel: string(c.NextChannel()),
		})
	}
	return out // 呼び出し側で安定ソート（CampaignID 昇順）
}
```

> map 反復は順不同。決定的な順序が要るので httpapi 側で `CampaignID` 昇順ソートする
> （テストの安定化のため）。

### httpapi
- `Deps` に `Dunning DunningLister` を追加（インタフェース `ListCampaigns() []CampaignView`）。
- ルート: `GET /api/dunning-campaigns`。
- `dto.go`: `dunningCampaignDTO`（camelCase へ写像）。
- `cmd/api/main.go`: 既に生成済みの `dunning` Service を `httpapi.New` の `Deps` に渡す。

### テスト（`httpapi_test.go`）
- `TestListDunningCampaigns`: 決済失敗イベントで起票 → 一覧に active が出る、
  入金イベントで `resolved` に変わる、進捗（StepsTriggered）が増える。

## P2.3 フロントエンド設計

- `types.ts`: `DunningCampaign` interface。
- `client.ts`: `listDunningCampaigns(): Promise<DunningCampaign[]>` を3実装に追加。
- Sidebar に項目「督促」を追加（`View` に `"dunning"`）。
- 画面: キャンペーン一覧テーブル（請求ID・状態 Pill・進捗 `2/3`・次チャネル）。
  - 「次のステップを進める」ボタン（任意・MVP外でも可）は `AdvanceCampaigns` を叩く
    `POST /api/dunning/advance` を足せば実現可能。本フェーズは**一覧（読み取り）優先**、
    操作ボタンは余裕があれば。

---

# P3: 入金・消込（settlement）

## P3.1 現状

| 区分 | 公開 API | 状態 |
|---|---|---|
| コマンド | `ImportBankDeposits(ctx, []DepositInput)` | ✅ 銀行入金取込＋自動照合 |
| コマンド | `ReconcileManually(ctx, invoice, amount)` | ✅ 手動消込 |
| 読み取り | — | ❌ 無い |

内部状態: `settlements`（消込実績）・`outstanding`（未消込の請求＝消込候補）・
`deposits`（取込入金）。自動照合に失敗した入金は `UnmatchedDepositDetected` を発行。
`Settlement{ID, Invoice, Amount, reconciled}`、`Candidate{Invoice, Account, PayerName, Outstanding}`。

## P3.2 バックエンド設計

### domain アクセサ追加（`settlement.go` domain）
`reconciled` が非公開なため:

```go
// Reconciled は充当済み額、Remaining は残額を返す（読み取り用）。
func (s *Settlement) Reconciled() shared.Money { return s.reconciled }
func (s *Settlement) Remaining() shared.Money  { return s.Amount.Sub(s.reconciled) }
```

> `Money.Sub` が無ければ shared に追加（`Add` と対称。負値・通貨不一致は既存方針に従う）。

### 公開 Service に読み取り追加（`settlement.go`）

```go
// SettlementView は消込実績の外向きビュー。
type SettlementView struct {
	SettlementID shared.SettlementID
	InvoiceID    shared.InvoiceID
	Amount       shared.Money // 入金額
	Reconciled   shared.Money // 充当済み
	FullyApplied bool         // 全額充当済みか（部分消込の可視化）
}

// OutstandingView は未消込の請求（消込候補）。手動消込画面の対象一覧。
type OutstandingView struct {
	InvoiceID   shared.InvoiceID
	Account     shared.BillingAccountID
	PayerName   string
	Outstanding shared.Money
}

func (s *Service) ListSettlements() []SettlementView { /* settlements を写像 */ }
func (s *Service) ListOutstanding() []OutstandingView { /* outstanding を写像 */ }
```

map 反復は httpapi 側で ID 昇順ソート。

### httpapi
- `Deps` に `Settlement SettlementReader` を追加。
- ルート:
  - `GET /api/settlements` … 消込実績一覧
  - `GET /api/settlements/outstanding` … 未消込（消込候補）一覧
  - `POST /api/bank-deposits` … 入金取込バッチ（`[]DepositInput` 相当の JSON）
  - `POST /api/settlements/manual` … 手動消込（`{invoiceId, amount}`）
- `dto.go`: `settlementDTO` / `outstandingDTO` / `depositInputDTO` / `manualReconcileRequest`。
- `cmd/api/main.go`: 既存の `settlement` Service を `Deps` に渡す。
- エラー写像: `ErrOverApplication`→409 `over_application`、`ErrCurrencyMismatch`→400、
  未知 invoice（候補に無い）→404。既存 `writeError` 様式に揃える。

### テスト（`httpapi_test.go`）
- `TestListSettlements`: 決済成功で消込が一覧に出る。
- `TestManualReconcile`: 未消込の請求へ `POST /api/settlements/manual` → 消込され
  `outstanding` から消える。過消込は 409。
- `TestImportBankDeposits`: 自動照合できる入金は消込、できないものは候補に残る。

## P3.3 フロントエンド設計

- `types.ts`: `Settlement` / `OutstandingInvoice` interfaces。
- `client.ts`: `listSettlements()` / `listOutstanding()` / `importBankDeposits(rows)` /
  `reconcileManually(invoiceId, amount)` を3実装に追加。
- Sidebar に項目「入金・消込」を追加（`View` に `"settlement"`）。
- 画面（2ペイン）:
  1. **未消込（消込候補）**: 請求ID・請求先・名義・残額。行から「手動消込」フォーム
     （金額入力 → `reconcileManually`）。
  2. **消込実績**: 消込ID・請求ID・入金額・充当済み・全額/部分バッジ。
  - 「銀行入金取込」: CSV 風の複数行入力 → `importBankDeposits`（MVP は数行のフォームで可）。

---

## 受け入れ条件（Given-When-Then 抜粋）

- **P2-AC1** Given 決済失敗で督促起票済み, When 督促画面を開く, Then active の
  キャンペーンが進捗付き（例 `1/3`・次=sms）で表示される。
- **P2-AC2** Given 督促中の請求が入金済みになる, When 一覧更新, Then 状態が `resolved`。
- **P3-AC1** Given 未消込の請求がある, When 入金・消込画面, Then 候補一覧に残額が出る。
- **P3-AC2** Given 候補へ残額と同額で手動消込, When 実行, Then 候補から消え消込実績に出る。
- **P3-AC3** Given 残額を超える手動消込, When 実行, Then `409 over_application`。

## 作業順序（P2 → P3 の順。各々で build/vet/test グリーン）

1. **P2 backend**: domain アクセサ → `ListCampaigns` → httpapi（Deps/route/dto/main 結線）→ テスト。
2. **P2 frontend**: types/client/Sidebar/画面。
3. **P3 backend**: `Money.Sub`（必要なら）→ domain アクセサ → `ListSettlements`/`ListOutstanding`
   → httpapi（read 2本 + command 2本）→ テスト。
4. **P3 frontend**: types/client/Sidebar/2ペイン画面。
5. PR（P2・P3 は別 PR 推奨。レビュー単位を小さく保つ）。

## 設計3原則との整合（CLAUDE.md）

- **入金は非同期**: settlement の `outstanding`（pending な債権）と `settlements`（確定）を
  別一覧で見せ、「まだ入っていない／入った」を画面で峻別する。これが本フェーズの核。
- **債権≠決済手段**: 督促・消込の一覧に決済手段 ID を出さない。督促はチャネル（連絡手段）で
  あって決済手段ではない。
- **モジュール境界**: 追加する `ListX()` は各モジュール**自身の公開 API**。httpapi は
  公開 Service のみ呼び、他モジュールの map・domain には触れない。domain アクセサは
  自モジュール domain 内に閉じ、`shared`＋標準ライブラリ以外に依存しない（depguard 準拠）。
