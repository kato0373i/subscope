# subscope ドメイン設計

サブスクリプション業務 SaaS（会費 Pay 型）のドメイン設計ドキュメント。

- **アーキテクチャ**: 厳格なモジュラーモノリス（DDD）
- **技術スタック**: フロントエンド = React + TypeScript / バックエンド = Go / DB = PostgreSQL
- **設計の核心**: 請求（債権）と決済手段を構造的に疎結合にし、決済手段の切替を含む回収戦略を独立ドメインとして表現する

---

## 1. 設計思想

日本の定期課金業務では決済手段がクレカに限らず、口座振替・払込票・振込など多様で、かつ**入金確定が非同期**になる。これを破綻なく扱うため、以下を分離する。

- **会員 ≠ 支払者** — サービスを受ける主体（Member）と、支払責任を負う主体（BillingAccount）を分ける。団体一括請求や代理支払いに対応するため。
- **債権 ≠ 決済手段** — Invoice（請求書）は「いくら回収すべきか」だけを知り、決済手段への参照を一切持たない。
- **決済実行を独立した試行として表現** — PaymentTransaction が Invoice と PaymentMethod を一回の試行として橋渡しする。
- **入金事実と消込を分離** — Settlement が「実際に入金された事実」を債権に適用する（特に口座振替・振込は後日確定するため）。
- **回収をオーケストレータとして独立** — Collection が戦略に従い、リトライ・決済手段の切替・督促エスカレーションを指揮する。

中核となる疎結合の連鎖:

> **Invoice（債権）** は「いくら回収すべきか」だけを知る。
> **PaymentMethod（手段）** は「どう払えるか」だけを知る。
> **PaymentTransaction（決済実行）** が両者を一回の試行として橋渡しする。
> **Settlement（消込）** が「実際に入金された事実」を債権に適用する。
> **Collection（回収）** がこれら全部を戦略に従って指揮する。

Invoice は `payment_method_id` を**絶対に持たない**。これが「請求と決済手段の疎結合」を構造的に保証する。

---

## 2. コンテキストマップ（モジュール全体像）

```
┌─────────────── organization (テナント境界) ───────────────┐
│                                                            │
│  member ──┐                          plan                  │
│           │                            │                   │
│           └──────► contract ◄──────────┘                   │
│                       │  (proration / trial を内包)         │
│                       ▼  BillingDue                         │
│                    billing ──► Invoice (債権) ──► tax        │
│                       │                                     │
│                       ▼  InvoiceIssued                      │
│                  collection (回収オーケストレータ)            │
│        ┌──────────────┼───────────────┐                    │
│        ▼              ▼               ▼                     │
│     payment      payment_method     dunning ──► notification│
│   (決済実行)      (決済手段の保管)     (督促フロー)            │
│        │                                                    │
│        ▼  PaymentSucceeded / 銀行データ取込                  │
│    settlement (入金・消込) ──► Invoice を消し込む            │
│                                                            │
│  coupon   metrics   audit   webhook   billing_account      │
└────────────────────────────────────────────────────────────┘
```

---

## 3. 「払う人」と「会員」の分離（会費 Pay 固有の論点）

会費ビジネスでは **会員 ≠ 支払者** が頻繁に起きる（団体が会員分を一括払い、親が子の会費を払う等）。

- **Member（会員）** … サービスを受ける主体。請求の宛先ではない。
- **BillingAccount（請求先 / 支払者）** … 請求書を受け取り、決済手段を保有し、支払責任を負う主体。1つの BillingAccount が複数 Member を束ねられる（＝団体一括請求）。

Contract は `member_id` と `billing_account_id` の**両方**を参照する。PaymentMethod は **BillingAccount に属する**（Member ではない）。

---

## 4. 各集約の詳細設計

表記ルール:

- **Root** = 集約ルート
- **E** = エンティティ
- **VO** = 値オブジェクト
- **🔗** = 他集約を ID 参照（直接参照禁止）
- **⚡** = 発行イベント
- **👂** = 購読イベント

### 4.1 contract（契約）— サブスクの心臓部

| 項目 | 内容 |
|---|---|
| **Root** | `Contract` |
| E | `ContractItem`（契約明細：plan × 数量。1契約に複数プラン可） |
| VO | `ContractStatus`（trialing/active/past_due/suspended/cancelled）, `BillingCycle`（請求サイクル）, `BillingAnchor`（起算日）, `TrialPeriod`, `Term`（開始/終了） |
| 🔗 | `member_id`, `billing_account_id`, `plan_id`（各 ContractItem） |
| **不変条件** | ・status 遷移は状態機械で制約（cancelled から active へ戻れない 等）<br>・`next_billing_date` は BillingAnchor と Cycle から導出され、常に未来<br>・trial 中は課金対象外 |
| ⚡ | `ContractActivated`, `BillingDue`（請求サイクル到来）, `PlanChanged`, `ContractCancelled`, `ContractSuspended` |

**proration / trial をここに内包する理由**: 日割りもトライアルも「契約状態の関数」であり、独立したライフサイクル（生成→消滅）を持たない。別集約にすると Contract と常にトランザクションを跨いで整合させる羽目になる。

- **Proration は集約ではなく Domain Service**: `ProrationPolicy.calculate(contract, changeDate)` → `調整明細(VO群)` を返す。これを billing が請求行に変換。
- **Trial は Contract の状態**: `TrialPeriod` VO ＋ status=trialing。トライアル終了は Contract が `BillingDue` を出すだけ。

> 整合性: `BillingDue` を契機に billing が Invoice を生成（**結果整合性**）。Contract と Invoice は別トランザクション。

### 4.2 billing（請求）— Invoice = 債権オブジェクト

| 項目 | 内容 |
|---|---|
| **Root** | `Invoice` |
| E | `InvoiceLine`（請求明細：金額・数量・税区分・proration 調整） |
| VO | `InvoiceStatus`（draft/issued/paid/partially_paid/void/uncollectible）, `Money`, `BillingPeriod`, `TaxBreakdown`（税率別内訳） |
| 🔗 | `contract_id`, `billing_account_id`, `coupon_id?`（適用済割引の記録）, `qualified_invoice_no`（適格請求書番号） |
| **不変条件** | ・`issued` 後は明細を変更不可（修正は赤伝＝CreditNote 別 Invoice で対応）<br>・`合計 = Σ明細 − 割引 + 税`<br>・status は単調に進む（paid から draft へ戻れない）<br>・**決済手段への参照を一切持たない** |
| ⚡ | `InvoiceIssued`（→ collection が拾う）, `InvoicePaid`, `InvoiceVoided`, `InvoiceUncollectible` |
| 👂 | `BillingDue`（Contract）, `CouponApplied` |

**重要**: Invoice は「誰がどう払うか」を知らない。`paid` への状態変更は **settlement からの `InvoicePaid` トリガ**でのみ起きる（自分では決済しない）。

### 4.3 tax（税 / インボイス制度）

集約というより**ポリシー＋登録番号管理**。

| 項目 | 内容 |
|---|---|
| **Root** | `TaxRegistration`（適格請求書発行事業者の登録番号、有効期間） |
| Domain Service | `TaxCalculator`（税率別＝10%/8%軽減の按分、端数処理ルール） |
| VO | `TaxRate`, `TaxCategory`, `TaxBreakdown` |
| 提供 | billing が請求確定時に呼び、`TaxBreakdown` を Invoice に焼き込む |

> インボイス制度の法的要件（登録番号・税率別金額・税額）は Invoice に**スナップショットとして保存**（後から税率が変わっても発行済請求書は不変）。

### 4.4 payment_method（決済手段）— 手段の保管庫、債権と無関係

| 項目 | 内容 |
|---|---|
| **Root** | `PaymentMethod`（ポリモーフィック） |
| 種別 | `CreditCard` / `BankAccount`(口座振替) / `PaymentSlip`(払込票・コンビニ) / `VirtualAccount`(振込用バーチャル口座) |
| VO | `PaymentMethodStatus`, `RegistrationStatus`, トークン参照（生 PAN 等は保持しない＝PSP トークンのみ） |
| 🔗 | `billing_account_id` |
| **不変条件** | ・口座振替は `RegistrationStatus`（依頼受付→銀行審査中→登録完了/否認）の状態機械を必ず通る<br>・登録未完了の手段は決済に使えない<br>・1つの BillingAccount は複数手段を持ち**優先順位**を付けられる |
| ⚡ | `PaymentMethodRegistered`, `BankAccountRegistrationCompleted`, `PaymentMethodExpired`（カード期限切れ） |

**口座振替の registration 状態機械**が日本特有の肝。「登録依頼を出した」≠「使える」。この時間差を型で表現する。

### 4.5 payment（決済実行）— Invoice と PaymentMethod を橋渡しする一回の試行

| 項目 | 内容 |
|---|---|
| **Root** | `PaymentTransaction`（＝1回の決済試行） |
| VO | `TransactionStatus`（requested/authorized/captured/failed/**pending**/refunded）, `Money`, `FailureReason`（残高不足/限度額/口座エラー…） |
| 🔗 | `invoice_id`, `payment_method_id`（**ここで初めて両者が出会う**） |
| ACL | PSP（Stripe / GMO / SB ペイメント等）への腐敗防止層。ゲートウェイ差異をここに閉じ込める |
| **不変条件** | ・1試行は1手段・1金額に固定（途中で手段を変えない＝変えるなら新トランザクション）<br>・`pending` を持てる（口座振替・払込票は**結果が後日確定**） |
| ⚡ | `PaymentSucceeded`, `PaymentFailed`, `PaymentPending` |
| 👂 | collection からの `ExecuteCharge` コマンド |

> **設計の急所**: クレカは同期で `captured`、口座振替・払込票は `pending` を返して**後日 settlement が確定させる**。この非同期性を Status で一級市民として扱う。

### 4.6 settlement（入金・消込）— 実際に入った金を債権に当てる

| 項目 | 内容 |
|---|---|
| **Root** | `Settlement`（入金事実）/ `ReconciliationEntry`（消込明細） |
| VO | `ReceivedAmount`, `SettlementSource`（PSP 通知 / 銀行入金データ / 手動）, `MatchStatus`（auto/manual/unmatched） |
| 🔗 | `invoice_id`, `payment_transaction_id?` |
| **不変条件** | ・`Σ消込額 ≤ 入金額`（過消込禁止）<br>・1入金が複数請求に按分可（団体一括の戻し込み）<br>・振込人名義の揺れ → **自動消込＋手動消込**のハイブリッド |
| ⚡ | `InvoicePaid`（全額消込）, `InvoicePartiallyPaid`, `UnmatchedDepositDetected`（要手動対応） |
| 👂 | `PaymentSucceeded`（クレカ即時）, 銀行入金データ取込バッチ |

> クレカ: `PaymentSucceeded` → 即 Settlement 自動生成。
> 口座振替/振込: 銀行データを取り込み、振込人名義・金額で Invoice とマッチング（消込）。ここが現場で一番泥臭い領域なので独立させる。

### 4.7 collection（回収）— 戦略を持つオーケストレータ ★中核

| 項目 | 内容 |
|---|---|
| **Root** | `CollectionCase`（未収となった Invoice 1件＝回収案件） |
| E | `CollectionAttempt`（各リトライの記録：いつ・どの手段で・結果） |
| VO | `CollectionStrategy`（後述）, `CaseStatus`（in_progress/recovered/escalated/written_off） |
| 🔗 | `invoice_id`, 現在試行中の `payment_method_id`（戦略が選んだもの） |
| **不変条件** | ・試行回数は戦略の上限を超えない<br>・全手段を試し尽くしたら `escalated`（人手 or 解約） |
| ⚡ | `ChargeRequested`（→ payment へコマンド）, `CollectionEscalated`, `CollectionRecovered`, `WrittenOff`（貸倒） |
| 👂 | `InvoiceIssued`（案件起票）, `PaymentFailed` / `PaymentSucceeded`, `InvoicePaid` |

**`CollectionStrategy`（回収戦略）の中身** — 「決済手段の変更を視野に入れた戦略」:

```
CollectionStrategy {
  retryPolicy:        { maxAttempts, interval(指数バックオフ/固定日付) }
  methodFallback:     [優先1: 登録カード → 失敗なら
                       優先2: 別カード     → 失敗なら
                       優先3: 口座振替再請求 → 失敗なら
                       優先4: 払込票送付(人が払う手段に切替)]
  escalationRule:     { 通知(dunning起動) → 利用停止 → 解約申請 }
  writeOffRule:       { 何日経過で貸倒計上 }
}
```

`CollectionCase` は失敗ごとに戦略を参照し、**次にどの `payment_method_id` で `ChargeRequested` を出すか**を決定する。手段の切替＝新しい `PaymentTransaction` の生成として表現され、Invoice は一切変わらない（疎結合が効く）。

> 戦略は組織ごと/プランごとに差し替え可能にする（年会費は厳しめ、月会費は緩め等）。

### 4.8 dunning（督促）— 「通知の流れ」を回収から分離

回収（金を取りに行く）と督促（人に知らせて促す）は責務が違うので分離する。

| 項目 | 内容 |
|---|---|
| **Root** | `DunningCampaign`（督促シナリオの進行状態） |
| E | `DunningStep`（D+0 メール → D+3 SMS → D+7 督促状…） |
| VO | `Channel`(email/SMS/郵送), `Template` |
| ⚡ | `DunningStepTriggered`（→ notification） |
| 👂 | `CollectionEscalated`, `PaymentFailed` |

### 4.9 その他の支援ドメイン

| モジュール | Root / 役割 | 備考 |
|---|---|---|
| **organization** | `Organization`（テナント）, `Operator`（SaaS 利用者の社員・権限） | 全集約に `org_id`。会員(Member)とは別物 |
| **member** | `Member`（会員）, `MemberStatus` | サービス受益者。請求先ではない |
| **billing_account** | `BillingAccount`（請求先/支払者） | 決済手段を所有。団体一括の束ね単位 |
| **plan** | `Plan`, `Price`(VO), `BillingInterval` | カタログ。発行済 Invoice には金額をスナップショット |
| **coupon** | `Coupon`, `Redemption`(E) | 初回無料・n ヶ月割引。billing が適用、二重利用を redemption で防止 |
| **metrics** | 読み取り専用の投影（MRR/ARR/Churn/LTV） | **書き込み集約ではなく CQRS の Read Model**。各イベントを集計 |
| **audit** | `AuditLog`（追記専用） | 金融性が高いので全コマンドを記録。不変 |
| **webhook** | `WebhookEndpoint`, `Delivery`(E) | 会計ソフト連携・Slack 通知。配信リトライを持つ |
| **notification** | メール/SMS 送信の実体 | dunning から駆動される下位サービス |

> ✅ **実装（#19）**: metrics / audit / webhook を最小スライスで新設。
> - **metrics**: `Projection`（Read Model）が `ContractActivated/Cancelled`・`InvoiceIssued/Paid`・`CollectionRecovered/WrittenOff`・`CreditNoteIssued` を購読集計し、`Snapshot()` で現在値を返す。書き込み集約ではなくイベント集計のみ。
> - **audit**: `events.AllNames()` で全統合イベントを購読し、`Entry`（不変・追記専用）として記録。フィールド変更メソッドを持たない。
> - **webhook**: `Endpoint`（購読イベント集合）と `Delivery`（pending→delivered/failed の状態機械）。配信は `Transport` ACL に閉じ込め、失敗時は上限までリトライ。配信失敗はバスを止めず記録に残す。
> - いずれも新たな統合イベントは発行しない（純粋な観測・連携の下流）。送信/配信の実体は ACL（`Transport`）に隔離し既定はモック。

---

## 5. エンドツーエンドの回収フロー（疎結合の実証）

```
① Contract: 請求日到来 → ⚡BillingDue
② billing:  Invoice生成(tax焼込) → ⚡InvoiceIssued      [Invoiceは手段を知らない]
③ collection: CollectionCase起票, 戦略選択
              → 優先1のpayment_method_idで ⚡ChargeRequested
④ payment:  PaymentTransaction生成(invoice_id × method_id)
            ├─ クレカ → captured → ⚡PaymentSucceeded
            └─ 口座振替 → pending → ⚡PaymentPending
⑤a settlement: PaymentSucceeded受領 → Settlement生成 → ⚡InvoicePaid
⑤b settlement: 後日、銀行データ取込 → 消込 → ⚡InvoicePaid / UnmatchedDeposit
⑥ billing:  InvoicePaid受領 → Invoice.status=paid
⑦ collection: InvoicePaid受領 → Case=recovered

【失敗時】
④' payment: PaymentFailed
⑤' collection: 戦略に従い次の手段へ → 新ChargeRequested(別method_id)
              上限到達 → ⚡CollectionEscalated → dunning起動 → 最終的に貸倒/解約
```

ここで Invoice は最初から最後まで**一度も決済手段を参照していない**。手段の切替は collection と payment の中だけで完結する。これが「請求と決済手段の疎結合」の構造的な証明。

---

## 6. 厳格なモジュラーモノリスの境界ルール

コンパイル時・実行時に**境界を破れない**仕組みを置く。

1. **公開 API は2種類だけ**
   - 同期: 各モジュールの `XxxModuleApi`（インターフェース）経由のクエリ/コマンドのみ。内部の集約・リポジトリは package-private。
   - 非同期: ドメインイベント（モジュール間は**イベント＋ID**でのみ会話）。
2. **他モジュールの集約を直接 import 禁止**。参照は必ず ID（`MemberId`, `InvoiceId`…）の VO。
3. **DB スキーマをモジュールごとに分離**（schema 分割 or テーブル prefix）。**他モジュールのテーブルへ JOIN 禁止**。必要なデータは API かイベントで取得しローカルに投影。
4. **1トランザクション＝1集約**。モジュール跨ぎは必ず結果整合性（Outbox パターンでイベント発行）。
5. Go なら **internal パッケージ＋依存方向の静的チェック**（例: `go-arch-lint` / `depguard`）で違反を CI で弾く。

### ディレクトリ構成（バックエンド）

```
backend/
  internal/
    contract/      ← api.go(公開), domain/(非公開), app/, infra/
    billing/
    payment/
    paymentmethod/
    collection/
    settlement/
    tax/
    dunning/
    organization/
    member/
    billingaccount/
    plan/
    coupon/
    metrics/
    audit/
    webhook/
    notification/
    shared/        ← Money, IDのVO, イベントバス抽象のみ（業務ロジック禁止）
    platform/      ← DB, eventbus, outbox の実装
  cmd/api/
```

各モジュールは `internal/<module>/` の外から domain を触れない（Go の `internal` 機構＋lint で強制）。

---

## 7. 先に決めておくべき論点

- **CreditNote（赤伝/返金）**: Invoice とは別集約にするか、Invoice の負債行で表すか → インボイス制度的には**別文書（適格返還請求書）**推奨。
  - ✅ **決定（#18）**: 独立集約 `creditnote.CreditNote` として実装。`issued → applied` の状態機械を持ち、紐付けは `ContractID` で行う（返金事由の発生源が `contract.PlanChanged`／解約で、いずれも契約単位のため。請求書単位の特定が必要になった段階で `InvoiceID` を追加する）。Invoice の不変条件（issued 後の明細変更不可）を壊さず、適格返還請求書として独立文書化できる。`contract.PlanChanged` の差額が負（ダウングレード返金）のとき自動発行し、解約返金等は手動 `Issue` で発行。`CreditNoteIssued` を発行する。
- **冪等性**: PSP からの Webhook 二重通知に備え、payment / settlement に冪等キー必須。
- **時刻と締め**: 請求の「締め日」概念（月末締め翌月請求）を Contract か billing のどちらに置くか。
  - ✅ **決定（#18）**: **Contract に配置**。締め日は請求サイクル（`BillingCycle` / `BillingAnchor`）と同じく契約の請求スケジュール属性であり、billing は「締められた期間」を受けて請求する関係のため。`contract` ドメインに `ClosingPolicy`（締め日 VO・月末締め対応の `ClosingDate`）を実装した。

---

## 8. 今後のステップ

- (A) 各集約のドメインイベント一覧とスキーマ（イベントカタログ）の詳細化
- (B) Go でのディレクトリ＆境界強制の実装スキャフォールド作成
- (C) 主要集約の ER / 状態遷移図
