# subscope

サブスクリプション業務 SaaS。サブスクリプション業務を AX する定期課金プラットフォーム。

## 特徴

- **厳格なモジュラーモノリス**（DDD）。モジュール間は型付き ID とドメインイベントでのみ会話する。
- **請求（債権）と決済手段の疎結合**。Invoice は決済手段への参照を一切持たない。
- **回収ドメイン**が、リトライ・決済手段の切替・督促エスカレーションを戦略として表現する。
- 日本の定期課金を想定し、クレカ・口座振替・払込票・振込（バーチャル口座）を同列に扱う。

## 技術スタック

| 層 | 技術 |
|---|---|
| フロントエンド | React + TypeScript（予定） |
| バックエンド | Go |
| DB | PostgreSQL（予定） |

## ディレクトリ構成

```
subscope/
├── docs/
│   └── domain-design.md      # ドメイン設計ドキュメント
├── backend/
│   ├── cmd/api/              # エントリポイント
│   └── internal/
│       ├── contract/         # 契約（状態機械・トライアル・日割り proration）
│       ├── billing/          # 請求（Invoice = 債権）
│       ├── collection/       # 回収（リトライ/手段切替・貸倒・エスカレーション戦略）
│       ├── payment/          # 決済実行（PaymentTransaction・PSP ACL・pending）
│       ├── settlement/       # 入金・消込（銀行入金取込・部分消込・按分・名寄せ）
│       ├── dunning/          # 督促（DunningCampaign / Step シーケンス）
│       ├── notification/     # 通知（email/SMS/郵送・Sender ACL）
│       ├── plan/             # プランマスタ（Plan / Price / BillingInterval・金額スナップショット）
│       ├── coupon/           # クーポンマスタ（Coupon / Redemption・二重利用防止）
│       ├── creditnote/        # 赤伝（適格返還請求書・返金）
│       ├── tax/              # 税（インボイス制度対応の税計算）
│       ├── metrics/          # 指標投影（CQRS Read Model：請求/回収/解約をイベント集計）
│       ├── audit/            # 監査ログ（全統合イベントを追記専用・不変で記録）
│       ├── webhook/          # 外部連携（WebhookEndpoint / Delivery・配信リトライ）
│       ├── shared/           # 型付き ID・Money・イベント抽象・統合イベント
│       └── platform/         # インフラ実装（イベントバス・HTTP API レイヤ）
└── frontend/                 # 管理画面（React + TypeScript + Vite）
    └── src/
        ├── App.tsx           # 画面（契約一覧・請求/回収状況）
        └── api/              # SubscopeApi 抽象 + MockApi / HttpApi（接続方針は frontend/README）
```

各モジュールは公開パッケージ（`internal/<module>/`）のみを外部に晒し、エンティティは
`internal/<module>/internal/domain/` に閉じる。これにより Go のコンパイラが
**他モジュールからのドメイン直接参照をコンパイルエラーとして禁止**する。
ドメイン層の純粋性（shared 以外に依存しない）は golangci-lint の depguard でも二重に強制する。

## 実行（最小スライスのデモ）

請求サイクル到来 → 請求書発行 → 回収案件起票 → 決済（主カード失敗 → 戦略で別手段へ切替）
→ 決済成功 → 入金消込 → 請求書を入金済みに更新、という縦串をインメモリで実演する。

```sh
cd backend
go run ./cmd/api
```

`go run ./cmd/api` はデモのシード投入後、既定で HTTP API サーバを `:8080` で常駐起動する
（`SUBSCOPE_ADDR` で変更可、`-serve=false` でデモ単発）。

## HTTP API（REST）

`internal/platform/httpapi` が各モジュールの公開 Service を REST で公開する。ハンドラは
公開 API のみを呼び、ドメイン集約・他モジュールのテーブルには触れない（読み取りは複数
モジュールの公開 API を合成して DTO を返す）。

| メソッド・パス | 説明 |
|---|---|
| `GET /healthz` | ヘルスチェック |
| `GET /api/contracts` | 契約一覧 |
| `POST /api/contracts` | 契約登録（コマンド） |
| `POST /api/contracts/{id}/billing` | 請求トリガ（コマンド） |
| `GET /api/invoices` | 請求書一覧 |
| `GET /api/collection-states` | 請求/回収状況（billing × collection を合成） |
| `GET /api/metrics` | 指標スナップショット |

フロントは `frontend/src/api` の `SubscopeApi` 抽象に依存し、`VITE_API_BASE_URL` を
設定すると `HttpApi`（実 API）、未設定なら `MockApi` を使う。

## 設計の詳細

[docs/domain-design.md](docs/domain-design.md) を参照。
