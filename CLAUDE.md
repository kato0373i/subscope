# subscope — 開発ルール（常時ロード）

subscope は「サブスクリプション業務を AX する会費 Pay 型 SaaS」。日本の定期課金（クレカ・口座振替・払込票・振込）を破綻なく扱うための**厳格なモジュラーモノリス**。

このファイルは**常時必要なルール**だけを持つ。作業別の詳細手順は [`playbooks/`](#playbooks-インデックス) に切り出してあり、必要なときだけ読む（コンテキスト/キャッシュ節約のため）。ドメインモデルの正典は [docs/domain-design.md](docs/domain-design.md)。

---

## 設計の背骨（常時遵守）

すべての規約は次の3原則から導かれる。判断に迷ったらここに立ち返る。

1. **会員 ≠ 支払者** — サービスを受ける `Member` と支払責任を負う `BillingAccount` を分ける。決済手段は `BillingAccount` に属する。
2. **債権 ≠ 決済手段** — `Invoice`（請求書＝債権）は「いくら回収すべきか」だけを持ち、**決済手段への参照を一切持たない**。`Invoice` に `payment_method_id` 相当を足したくなったら設計違反のサイン。
3. **入金は非同期** — 口座振替・払込票・振込は後日確定。`payment` は `pending` を一級市民として持ち、`settlement` が入金事実を債権に適用する。

## 絶対ルール（常時遵守）

- **モジュール境界** — 他モジュールとの会話は **型付き ID**（`internal/shared/id.go`）と **ドメインイベント**（`internal/shared/events`）の2経路のみ。他モジュールの集約・内部型を直接触らない。
- **ドメイン層の純粋性** — `internal/*/internal/domain` は `shared` と標準ライブラリ以外に依存しない（`.golangci.yml` の depguard `domain-purity` で CI 強制）。
- **整合性の境界** — 1トランザクション = 1集約。モジュール跨ぎは結果整合性（イベント）。他モジュールのテーブルへ JOIN しない。
- **金額** — `shared.Money`（`int64`・最小通貨単位）。浮動小数禁止。加算は `Money.Add`。消費税は税率ごとに1回だけ端数処理（`tax.Calculator`）。
- **整形** — Go はタブインデント（gofmt 準拠）。スペースは CI で落ちる。
- 変更後は `cd backend && go build ./... && go vet ./... && go test ./...` を通す。

---

## モデルルーティング

タスク開始時に種別を**自分で判断し、使うモデルを宣言してから**着手する。

| 作業 | モデル |
|---|---|
| 設計レビュー・アーキテクチャ検討・API 仕様策定 | `claude-opus-4-5` |
| 仕様確定後の実装・テスト・リファクタリング | `claude-sonnet-4-5` |

- 迷ったらまず opus で設計を固め、確定後 sonnet に切り替える。
- 宣言例：「これは API 仕様策定なので claude-opus-4-5 で進めます」。

## サブエージェント委譲

- **コードベース調査・レビュー・影響範囲分析はサブエージェントに委譲する**。メインコンテキストで総当たり読みをしない。
- サブエージェントは**結果の要約（結論・該当ファイル・該当箇所）だけ**をメインに返す。生のファイルダンプを持ち込まない。
- ファイルの総当たり読み込みより、**要約・抽出**を優先する。

## コード検索（semble 優先）

- **コード検索はまず semble を使う**。`semble` の `search`（自然言語/コードクエリ）と `find_related`（類似箇所）でセマンティック検索する。
- **Grep / Read による総当たりの前に、必ず semble で意味検索する**。総当たりは semble で当たりを付けた後の確認に限る。
- semble 未インストール時のセットアップは [playbooks/code-search.md](playbooks/code-search.md)。

## キャッシュ運用

- **`CLAUDE.md` やモデル設定（`.mcp.json` 等）を変更したら、必ずセッションを終了する**。変更はキャッシュを破棄するため、複数の変更はまとめて1回で行う。
- **セッション開始時はキャッシュが有効な状態で作業を始める**。開始直後に設定ファイルを書き換えない。

---

## playbooks インデックス

作業に入るときだけ該当ファイルを読む。

| いつ読むか | playbook |
|---|---|
| モジュール/集約/イベントを追加・変更する、境界の型を確認する | [playbooks/module-design.md](playbooks/module-design.md) |
| ブランチを切る・コミット・PR・CI・CodeRabbit 対応・マージ | [playbooks/git-workflow.md](playbooks/git-workflow.md) |
| コード検索の方針、semble のセットアップ・使い方 | [playbooks/code-search.md](playbooks/code-search.md) |
| 新規実装前に踏まえる未着手の論点 | [playbooks/open-questions.md](playbooks/open-questions.md) |

参照実装：境界の作り方は `internal/tax`、イベント駆動の回収戦略は `internal/collection`。
