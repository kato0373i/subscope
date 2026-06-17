# P1 実装計画: 顧客360 + 請求個票

> 管理画面拡充ロードマップ（日次オペ優先）の第1フェーズ。
> 1契約を開くと「契約ヘッダ + 請求履歴 + 各請求の回収ステータス + サマリ」が
> 1画面で見える**顧客個票（顧客360）**を追加する。

## 1. 目的とスコープ

### 狙い
今のフロントは契約「一覧」しか無く、1契約の請求・回収の縦串が見えない。
運用担当が「この契約、いま請求どうなってる？回収は？」を1画面で把握できる状態を作る。
このフェーズは**既存の読み取り API の合成のみ**で完結させ、バックエンドのドメイン／
公開 Service には手を入れない（費用対効果を最大化する）。

### やること
- httpapi に契約個票の合成エンドポイントを1本追加（読み取り合成層の責務）。
- frontend に契約詳細パネル（一覧の行クリックで開く）を追加。

### やらないこと（スコープ外・後続フェーズ）
- **回収の詳細**（失敗理由・リトライ回数・決済手段・次回リトライ日時）は出さない。
  根拠: `collection.CaseView` が `{InvoiceID, Status}` の2フィールドしか持たず、
  詳細を読む公開 API が無い。→ **P4（payment 明細）/ CaseView 拡張**で対応。
- 入金・消込の明細（settlement）→ **P3**。
- 督促の進行（dunning）→ **P2**。
- 部分入金（`partially_paid`）→ ドメインに部分入金概念が未実装（既存コメント #11/#44）。

## 2. 既存資産の確認（調査済み）

| 必要な読み取り | 公開 API | 充足 |
|---|---|---|
| 契約ヘッダ（会員名・月額・状態） | `contract.Service.List()` + `member.Service.Name(id)` | ✅ |
| 契約に紐づく請求書一覧 | `billing.Service.ListInvoices()`（`InvoiceView.ContractID` で絞れる） | ✅ |
| 各請求の回収ステータス | `collection.Service.ListCases()`（`InvoiceID` で join） | ✅ |

`InvoiceView` が `ContractID` を持つため、契約IDで請求書を束ねられる。
ステータス合成は既存 `collectionStatusFor()`（dto.go:156）をそのまま再利用する。

→ **バックエンドのドメイン変更ゼロ。httpapi の合成 + frontend のみ。**

> 補足: `contract.Service` に `Get(id)` は無いが、`List()` を id でフィルタすれば足りる
> （契約数は小さい前提のインメモリ実装）。`Get(id)` の追加は最適化として将来検討（本フェーズでは不要）。

## 3. バックエンド設計（httpapi）

### 3.1 エンドポイント

```text
GET /api/contracts/{id}
```

1契約の個票を1レスポンスで返す（複数往復を避ける合成 DTO）。
存在しない id は `404 not_found`（既存 `handleTriggerBilling` のエラー様式に揃える）。

### 3.2 DTO（`dto.go` に追記）

```go
// customerDetailDTO は顧客個票（顧客360）の合成レスポンス。
// contract / billing / collection の読み取りを契約単位に束ねる。
type customerDetailDTO struct {
	Contract contractDTO              `json:"contract"`
	Invoices []invoiceCollectionRow   `json:"invoices"`
	Summary  customerSummaryDTO       `json:"summary"`
}

// invoiceCollectionRow は請求書1行に回収ステータスを合成したもの。
// invoiceStatus は billing 由来の生ステータス、collectionStatus は
// billing×collection を合成した画面用ステータス（既存 collectionStatusFor）。
type invoiceCollectionRow struct {
	InvoiceID        string   `json:"invoiceId"`
	Amount           moneyDTO `json:"amount"`
	InvoiceStatus    string   `json:"invoiceStatus"`
	CollectionStatus string   `json:"collectionStatus"`
}

// customerSummaryDTO は個票上部に出す集計。outstanding=未入金合計、paid=入金済合計。
type customerSummaryDTO struct {
	InvoiceCount int      `json:"invoiceCount"`
	Paid         moneyDTO `json:"paid"`         // collectionStatus == "paid" の合計
	Outstanding  moneyDTO `json:"outstanding"`  // paid 以外の合計（債権残）
	InCollection int      `json:"inCollection"` // collectionStatus == "in_collection" の件数
}
```

### 3.3 ハンドラ（`httpapi.go` に追記）

```go
mux.HandleFunc("GET /api/contracts/{id}", s.handleGetContract) // 追加
```

処理:
1. `r.PathValue("id")` を取得。空なら 400。
2. `Contracts.List()` を id でフィルタ。無ければ 404 `not_found`。
3. `Invoices.ListInvoices()` を `ContractID == id` で絞る。
4. `Cases.ListCases()` を `map[InvoiceID]caseStatus` 化（既存 collection-states と同型）。
5. 各 invoice を `collectionStatusFor()` で合成し `invoiceCollectionRow` を作る。
6. サマリを集計（`shared.Money.Add` で加算。通貨混在は当面想定せず先頭通貨に揃える）。
7. `member.Name()` で会員名を解決し `contractDTO` を組む。

> ルーティング注意: Go 1.22+ の `http.ServeMux` は `GET /api/contracts/{id}` と
> 既存 `GET /api/contracts`・`POST /api/contracts/{id}/billing` が**競合しない**
> （メソッド＋セグメント数で区別される）。確認のうえ追加する。

### 3.4 テスト（`httpapi_test.go` に追記）

- `TestGetContract_Composition`: 契約 + 複数請求（paid / in_collection / issued 混在）を
  シードし、`GET /api/contracts/{id}` のレスポンスで
  - 各行の `collectionStatus` が期待どおり合成される
  - `summary.paid` / `summary.outstanding` の金額が正しい
  - `summary.inCollection` の件数が正しい
- `TestGetContract_NotFound`: 未知 id → 404 `not_found`。

## 4. フロントエンド設計

### 4.1 型（`api/types.ts` に追記）

```ts
/** 請求書1行 + 回収ステータス（顧客個票用）。 */
export interface InvoiceCollectionRow {
  invoiceId: string;
  amount: Money;
  invoiceStatus: string;
  collectionStatus: CollectionStatus;
}

/** 顧客個票（顧客360）。GET /api/contracts/{id} に対応。 */
export interface CustomerDetail {
  contract: Contract;
  invoices: InvoiceCollectionRow[];
  summary: {
    invoiceCount: number;
    paid: Money;
    outstanding: Money;
    inCollection: number;
  };
}
```

### 4.2 API クライアント（`api/client.ts`）

`SubscopeApi` インタフェースに追加し、3実装を埋める:

```ts
getCustomerDetail(contractId: string): Promise<CustomerDetail>;
```

- `HttpApi`: `this.get(`/api/contracts/${encodeURIComponent(contractId)}`)`。
- `MockApi`: シードした契約・請求からその場で合成して返す（オフラインでも画面確認可能に）。

### 4.3 画面（`App.tsx` + 新コンポーネント）

- 既存「契約」一覧（および「ダッシュボード」の契約テーブル）の**行クリックで詳細を開く**。
- 表示形態は**右からのスライドパネル（ドロワー）**を推奨。
  - 理由: Sidebar のナビ4項目（dashboard/operations/contracts/collections）を増やさず、
    一覧 → 個票の導線が自然。`View` 型の拡張も不要。
  - 代替案: `View` に `"contractDetail"` を足し選択中 id を state で持つ全画面遷移。
    実装は重め。本フェーズはドロワーで軽く入れる。
- パネル構成:
  1. ヘッダ: 契約ID・会員名・状態 Pill（既存 `StatusPill`）・月額。
  2. サマリ: 入金済 / 債権残 / 回収中件数（既存 `MetricCard` を流用可）。
  3. 請求履歴テーブル: 請求ID・金額・回収ステータス Pill。
- ローディング／エラー／空状態は既存 App の `loading`/`error`/`toast` 様式に合わせる。

### 4.4 新規ファイル

- `frontend/src/components/CustomerDrawer.tsx`（個票パネル）。
- 既存 `StatusPill` / `MetricCard` / icons を再利用。

## 5. 受け入れ条件（Given-When-Then）

- **AC-1** Given デモデータ投入済み, When 契約 CT-0001 の行をクリック,
  Then 右ドロワーに会員名・状態・月額と請求履歴が表示される。
- **AC-2** Given ある請求が入金済み, When 個票を開く,
  Then その行の回収ステータスが `paid`、サマリの「入金済」に金額が反映される。
- **AC-3** Given 回収中の請求がある, When 個票を開く,
  Then サマリ「回収中件数」が 1 以上、該当行が `in_collection`。
- **AC-4** Given 未知の契約ID, When `GET /api/contracts/UNKNOWN`,
  Then `404 not_found`。

## 6. 作業順序

1. backend: `dto.go` に DTO 追加 → `httpapi.go` にハンドラ・ルート追加。
2. backend: `httpapi_test.go` にテスト2本追加 → `go build ./... && go vet ./... && go test ./...` グリーン。
3. frontend: `types.ts` → `client.ts`（3実装）→ `CustomerDrawer.tsx` → `App.tsx` 結線。
4. frontend: `npm run build` で型エラーなし、Docker 再ビルドで動作確認（行クリック→ドロワー）。
5. PR 作成（`feat(httpapi/frontend): 顧客360（契約個票）を追加`）。

## 7. 設計3原則との整合（CLAUDE.md）

- **会員≠支払者**: 個票は Contract 起点。会員名は `member.Name()` 経由で取得し、
  BillingAccount/決済手段はこの画面に**出さない**（混同を持ち込まない）。
- **債権≠決済手段**: 請求行は金額と回収ステータスのみ。`payment_method_id` 相当は一切持たない。
- **モジュール境界**: ハンドラは各モジュールの**公開 Service のみ**を呼び、合成は読み取り層
  （httpapi）で行う。他モジュールのテーブル JOIN・内部型参照はしない。
