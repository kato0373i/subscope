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
│       ├── shared/           # 型付き ID・Money・イベント抽象・統合イベント
│       └── platform/         # インフラ実装（イベントバス等）
└── frontend/                 # React アプリ（予定）
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

## 設計の詳細

[docs/domain-design.md](docs/domain-design.md) を参照。
