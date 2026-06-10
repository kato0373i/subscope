# playbook: モジュール設計・追加

モジュール/集約/ドメインイベントを追加・変更するときに読む。常時遵守の原則は [CLAUDE.md](../CLAUDE.md) を参照。正典は [docs/domain-design.md](../docs/domain-design.md)。

## ディレクトリの型

各モジュールは「公開パッケージ + private ドメイン」の2層で構成する。

```text
backend/internal/<module>/
├── <module>.go              # 公開 API（package <module>）。Service と統合イベント購読を置く
└── internal/
    └── domain/              # private。集約・エンティティ・VO・不変条件・状態機械
        ├── <entity>.go
        └── <entity>_test.go
```

`internal/<module>/internal/domain` は Go の `internal` 機構により、**他モジュールから import するとコンパイルエラー**になる。これが境界の物理的な強制。公開パッケージ（`internal/<module>/`）だけが外から見える。

## モジュール間の会話は2経路だけ

他モジュールの集約（構造体）を直接触ることは禁止。会話は次の2つに限る。

- **型付き ID** — `shared.InvoiceID`, `shared.MemberID` など（`internal/shared/id.go`）。他モジュールを参照するときは必ず ID で。
- **ドメインイベント** — `internal/shared/events` の統合イベント。発行（`Publish`）と購読（`Subscribe`）でのみ非同期に会話する。

直接の関数呼び出しで他モジュールの内部に踏み込まない。同期的に他モジュールの問い合わせが必要なら、その公開パッケージのインターフェース越しに限る。

## ドメイン層の純粋性

`internal/*/internal/domain` は **`shared` と標準ライブラリ以外に依存しない**。インフラ（`platform`）や他モジュールに依存していたら設計違反。`.golangci.yml` の depguard `domain-purity` ルールで CI でも弾かれる。

## 整合性の境界

- **1トランザクション = 1集約**。複数集約をまたぐ更新を1トランザクションに入れない。
- モジュールをまたぐ整合は**結果整合性**で。イベントを発行し、購読側が自分の集約を更新する。
- DB はモジュールごとにスキーマ/テーブルを分け、**他モジュールのテーブルへ JOIN しない**。

## 金額（Money）の扱い

- 金額は `shared.Money`（`Amount int64`・最小通貨単位=円, `Currency`）。浮動小数は使わない。
- 加算は `Money.Add`（通貨不一致・オーバーフローでエラー）。生の `+` で `Amount` を足さない。
- 消費税は税区分（標準10%/軽減8%/非課税）ごとに合算してから**税率ごとに1回だけ**端数処理する（インボイス制度要件）。`tax` モジュールの `Calculator` を使う。

## 新しいモジュールを追加する手順

例として `dunning`（督促）を足す場合。既存の `tax` / `collection` が良い参照実装。

1. **設計を確認** — [docs/domain-design.md](../docs/domain-design.md) で集約・不変条件・発行/購読イベントを確認。なければ先に設計を追記する。
2. **必要なら型付き ID を追加** — `internal/shared/id.go` に `DunningCampaignID` 等を足す。
3. **統合イベントを定義** — モジュールをまたいで流れるイベントのみ `internal/shared/events/events.go` に追加（名前定数 + struct + `EventName()`）。モジュール内部だけで完結するイベントはここに出さない。
4. **ドメインを書く** — `internal/dunning/internal/domain/` に集約・VO・状態機械を置く。依存は `shared` と標準ライブラリのみ。不変条件と状態遷移は `*_test.go` で固定する。
5. **公開パッケージを書く** — `internal/dunning/dunning.go`（`package dunning`）。`Service` を定義し、`NewService(bus shared.EventBus)` で必要なイベントを `Subscribe` する。外部に見せる内部型は**型エイリアスで再エクスポート**して重複を避ける（`tax/tax.go` を参照）。
6. **結線** — `cmd/api/main.go` で `dunning.NewService(bus)` を呼ぶ。モジュール同士は `main` でバスに繋ぐだけ。
7. **README 更新** — `README.md` の構成図にモジュールを追記。
8. **検証** — `cd backend && go build ./... && go vet ./... && go test ./...`。可能なら `golangci-lint run` で境界チェックも。

## 状態を持つドメインは状態機械で守る

口座振替の登録（依頼受付→銀行審査→登録完了/否認）や契約・請求のステータスのように、遷移ルールがあるものは状態を単調に進め、不正な遷移を弾くメソッドとして表現する（例：`Invoice.MarkPaid` は `issued` からのみ `paid` へ）。状態フィールドを外から自由に書き換えさせない。
