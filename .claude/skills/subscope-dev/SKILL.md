---
name: subscope-dev
description: >-
  subscope（サブスクリプション業務・会費 SaaS）リポジトリでの開発規約とワークフロー。
  Go バックエンドの厳格なモジュラーモノリス構造（モジュールの追加方法、境界の強制、
  ドメイン層の純粋性、請求＝債権と決済手段の疎結合）と、GitHub Flow + CI + CodeRabbit を
  使った開発フローを定義する。subscope リポジトリで新しいモジュール・集約・ドメインイベントを
  追加する、backend/internal 配下の Go コードを書く・変更する、ブランチを切る・PR を作る・
  マージする、CodeRabbit の指摘に対応する、といった場面では必ずこのスキルを参照すること。
  モジュール境界・イベント駆動・回収戦略・決済手段の扱いに迷ったときも参照する。
---

# subscope 開発フレームワーク

subscope は「サブスクリプション業務を AX する会費 Pay 型 SaaS」。日本の定期課金（クレカ・口座振替・払込票・振込）を破綻なく扱うために、**厳格なモジュラーモノリス**として設計されている。このスキルはその規約と日々の開発フローをまとめたもの。

ドメインモデルの全体像（集約・不変条件・コンテキストマップ）は [docs/domain-design.md](../../../docs/domain-design.md) が正典。このスキルは「コードを書く・PR を出す」ときの実務ルールに焦点を当てる。

---

## 1. 設計の背骨（なぜこの構造か）

すべての規約は次の3原則から導かれる。新しい判断に迷ったら、ここに立ち返る。

1. **会員 ≠ 支払者** — サービスを受ける `Member` と、支払責任を負う `BillingAccount` を分ける。団体一括請求や代理支払いがあるため。決済手段は `BillingAccount` に属する。
2. **債権 ≠ 決済手段** — `Invoice`（請求書＝債権）は「いくら回収すべきか」だけを持ち、**決済手段への参照を一切持たない**。手段の切替は `collection`（回収）と `payment`（決済実行）の中だけで起きる。`Invoice` に `payment_method_id` 相当のフィールドを足したくなったら、それは設計違反のサイン。
3. **入金は非同期** — クレカは即時確定だが口座振替・払込票・振込は後日確定する。`payment` の `PaymentTransaction` は `pending` を一級市民として持ち、`settlement`（入金・消込）が実際の入金事実を債権に適用する。

---

## 2. モジュラーモノリスの構造と境界

### ディレクトリの型

各モジュールは「公開パッケージ + private ドメイン」の2層で構成する。

```
backend/internal/<module>/
├── <module>.go              # 公開 API（package <module>）。Service と統合イベント購読を置く
└── internal/
    └── domain/              # private。集約・エンティティ・VO・不変条件・状態機械
        ├── <entity>.go
        └── <entity>_test.go
```

`internal/<module>/internal/domain` は Go の `internal` 機構により、**他モジュールから import するとコンパイルエラー**になる。これが境界の物理的な強制。公開パッケージ（`internal/<module>/`）だけが外から見える。

### モジュール間の会話は2経路だけ

他モジュールの集約（構造体）を直接触ることは禁止。会話は次の2つに限る。

- **型付き ID** — `shared.InvoiceID`, `shared.MemberID` など（`internal/shared/id.go`）。他モジュールを参照するときは必ず ID で。
- **ドメインイベント** — `internal/shared/events` の統合イベント。発行（`Publish`）と購読（`Subscribe`）でのみ非同期に会話する。

直接の関数呼び出しで他モジュールの内部に踏み込まない。同期的に他モジュールの問い合わせが必要なら、その公開パッケージのインターフェース越しに限る。

### ドメイン層の純粋性

`internal/*/internal/domain` は **`shared` と標準ライブラリ以外に依存しない**。インフラ（`platform`）や他モジュールに依存していたら設計違反。これは `.golangci.yml` の depguard `domain-purity` ルールで CI でも弾かれる。ドメインを純粋に保つと、テストが速く・状態遷移や不変条件の検証がしやすくなる。

### 整合性の境界

- **1トランザクション = 1集約**。複数集約をまたぐ更新を1トランザクションに入れない。
- モジュールをまたぐ整合は**結果整合性**で。イベントを発行し、購読側が自分の集約を更新する。
- DB はモジュールごとにスキーマ/テーブルを分け、**他モジュールのテーブルへ JOIN しない**。

### 金額（Money）の扱い

- 金額は `shared.Money`（`Amount int64`・最小通貨単位=円, `Currency`）。浮動小数は使わない。
- 加算は `Money.Add`（通貨不一致・オーバーフローでエラー）。生の `+` で `Amount` を足さない。
- 消費税は税区分（標準10%/軽減8%/非課税）ごとに合算してから**税率ごとに1回だけ**端数処理する（インボイス制度要件）。`tax` モジュールの `Calculator` を使う。

---

## 3. 新しいモジュールを追加する手順

例として `dunning`（督促）を足す場合。既存の `tax` / `collection` が良い参照実装。

1. **設計を確認** — [docs/domain-design.md](../../../docs/domain-design.md) で集約・不変条件・発行/購読イベントを確認。なければ先に設計を追記する。
2. **必要なら型付き ID を追加** — `internal/shared/id.go` に `DunningCampaignID` 等を足す。
3. **統合イベントを定義** — モジュールをまたいで流れるイベントのみ `internal/shared/events/events.go` に追加（名前定数 + struct + `EventName()`）。モジュール内部だけで完結するイベントはここに出さない。
4. **ドメインを書く** — `internal/dunning/internal/domain/` に集約・VO・状態機械を置く。依存は `shared` と標準ライブラリのみ。不変条件と状態遷移は `*_test.go` で固定する。
5. **公開パッケージを書く** — `internal/dunning/dunning.go`（`package dunning`）。`Service` を定義し、`NewService(bus shared.EventBus)` で必要なイベントを `Subscribe` する。外部に見せる内部型は**型エイリアスで再エクスポート**して重複を避ける（`tax/tax.go` を参照）。
6. **結線** — `cmd/api/main.go` で `dunning.NewService(bus)` を呼ぶ。モジュール同士は `main` でバスに繋ぐだけ。
7. **README 更新** — `README.md` の構成図にモジュールを追記。
8. **検証** — `cd backend && go build ./... && go vet ./... && go test ./...`。可能なら `golangci-lint run` で境界チェックも。

### 状態を持つドメインは状態機械で守る

口座振替の登録（依頼受付→銀行審査→登録完了/否認）や契約・請求のステータスのように、遷移ルールがあるものは状態を単調に進め、不正な遷移を弾くメソッドとして表現する（例：`Invoice.MarkPaid` は `issued` からのみ `paid` へ）。状態フィールドを外から自由に書き換えさせない。

---

## 4. 開発ワークフロー（GitHub Flow）

main は常にデプロイ可能・CI グリーンを保つ。作業はすべてブランチ + PR で行う。

### ブランチ

main から切る。Conventional Commits に揃えた接頭辞を使う：

- `feat/<topic>` — 機能追加（例 `feat/tax-module`）
- `fix/<topic>` — バグ修正
- `chore/<topic>` — 設定・雑務（例 `chore/coderabbit-config`）

### コミット

Conventional Commits 形式。件名は日本語で良い。本文で「何を・なぜ」を簡潔に。末尾に必ず：

```
Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
```

**例：**
- `feat(tax): インボイス制度対応の税計算モジュールを追加`
- `fix(shared): Money.Add で int64 オーバーフローを検出する`
- `chore: CodeRabbit 設定を追加`

Go ファイルのインデントは**タブ**（gofmt 準拠）。スペースインデントは CI の gofmt で落ちる。

### PR

`gh pr create --base main` で作る。本文は構造化して、レビュアー（人間 + CodeRabbit）が読みやすくする：

- **## 概要** — 何のための変更か
- **## 変更内容** — 箇条書き or 表
- **## 設計上のポイント** — 上記3原則との関係、あえて残したトレードオフ（レビューで指摘してほしい点を明記すると有用）
- 末尾に `🤖 Generated with [Claude Code](https://claude.com/claude-code)`

### CI（必須グリーン）

`.github/workflows/ci.yml` が backend で `go build` / `go vet` / `go test` / `golangci-lint`（境界チェック）を走らせる。マージ前にグリーンを確認する。`gh run watch <run-id> --exit-status` で待てる。

> 注：依存ゼロのうちは「go.sum が無い」というキャッシュ警告と Node20 非推奨の注釈が出るが無害。

### CodeRabbit

- `.coderabbit.yaml` の設定で**日本語・chill プロファイル・自動レビュー**が効く。`path_instructions` でモジュール境界の文脈を渡しているので、境界違反や不適切な結合を拾いやすい。
- **インクリメンタルレビュー** — PR に新コミットを push すると、その差分を自動でレビューする。`@coderabbitai review` は手動再実行用で、**既にレビュー済みのコミットは再レビューしない**（「Review finished. ...already reviewed commits」が返ったら＝新規指摘なし＝OK のサイン）。
- 指摘への対応は**同じブランチに修正コミットを push** するだけ。CodeRabbit が差分を自動で見直す。妥当でない指摘は理由を添えてスキップしてよい。
- `.coderabbit.yaml` はベースブランチ（main）の内容が適用される。設定変更はマージして初めて有効になる。

### マージ

CI グリーン & レビュー解決後、**squash マージ + ブランチ削除**で main をクリーンに保つ：

```
gh pr merge <n> --squash --delete-branch
```

---

## 5. 検討事項・未着手の論点

新規実装の前に、関連するものはここを意識する（詳細は docs/domain-design.md §7）。

- **CreditNote（赤伝/返金）** — インボイス制度的には適格返還請求書として別文書が推奨。`Money.IsNegative` は返金判定用に用意済み。
- **冪等性** — PSP の Webhook 二重通知に備え、`payment` / `settlement` には冪等キーが要る。
- **Outbox パターン** — 現状のイベントバスはインメモリ・同期実装（`platform/eventbus`）。本番ではトランザクション境界を守るため Outbox + メッセージング基盤に差し替える。
- **締め日** — 月末締め翌月請求のような締め概念を `contract` / `billing` のどちらに置くか。
- **永続化** — 現状は in-memory map。PostgreSQL + マイグレーション + モジュール別スキーマへ。

---

## 6. クイックリファレンス

```sh
# ビルド・検証（backend ディレクトリで）
go build ./... && go vet ./... && go test ./...

# デモ実行（請求〜回収フローの縦串）
go run ./cmd/api

# 開発フロー
git switch -c feat/<topic>           # main から分岐
#   ...実装・テスト...
git push -u origin feat/<topic>
gh pr create --base main             # 構造化した本文で
gh run watch <run-id> --exit-status  # CI グリーン確認
#   CodeRabbit の指摘に対応（同ブランチに push）
gh pr merge <n> --squash --delete-branch
```

参照実装：境界の作り方は `internal/tax`、イベント駆動の回収戦略は `internal/collection` を見る。
