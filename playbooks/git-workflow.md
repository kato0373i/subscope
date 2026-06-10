# playbook: 開発ワークフロー（GitHub Flow）

ブランチを切る・コミット・PR・CI・CodeRabbit 対応・マージのときに読む。main は常にデプロイ可能・CI グリーンを保つ。作業はすべてブランチ + PR で行う。

## ブランチ

main から切る。Conventional Commits に揃えた接頭辞を使う：

- `feat/<topic>` — 機能追加（例 `feat/tax-module`）
- `fix/<topic>` — バグ修正
- `chore/<topic>` — 設定・雑務（例 `chore/coderabbit-config`）

## コミット

Conventional Commits 形式。件名は日本語で良い。本文で「何を・なぜ」を簡潔に。末尾に必ず：

```text
Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>
```

**例：**
- `feat(tax): インボイス制度対応の税計算モジュールを追加`
- `fix(shared): Money.Add で int64 オーバーフローを検出する`
- `chore: CodeRabbit 設定を追加`

Go ファイルのインデントは**タブ**（gofmt 準拠）。スペースインデントは CI の gofmt で落ちる。

## PR

`gh pr create --base main` で作る。本文は構造化して、レビュアー（人間 + CodeRabbit）が読みやすくする：

- **## 概要** — 何のための変更か
- **## 変更内容** — 箇条書き or 表
- **## 設計上のポイント** — 設計3原則（[CLAUDE.md](../CLAUDE.md)）との関係、あえて残したトレードオフ（レビューで指摘してほしい点を明記すると有用）
- 末尾に `🤖 Generated with [Claude Code](https://claude.com/claude-code)`

## CI（必須グリーン）

`.github/workflows/ci.yml` が backend で `go build` / `go vet` / `go test` / `golangci-lint`（境界チェック）を走らせる。マージ前にグリーンを確認する。`gh run watch <run-id> --exit-status` で待てる。

> 注：依存ゼロのうちは「go.sum が無い」というキャッシュ警告と Node20 非推奨の注釈が出るが無害。

## CodeRabbit

- `.coderabbit.yaml` の設定で**日本語・chill プロファイル・自動レビュー**が効く。`path_instructions` でモジュール境界の文脈を渡しているので、境界違反や不適切な結合を拾いやすい。
- **インクリメンタルレビュー** — PR に新コミットを push すると、その差分を自動でレビューする。`@coderabbitai review` は手動再実行用で、**既にレビュー済みのコミットは再レビューしない**（「Review finished. ...already reviewed commits」が返ったら＝新規指摘なし＝OK のサイン）。
- 指摘への対応は**同じブランチに修正コミットを push** するだけ。CodeRabbit が差分を自動で見直す。妥当でない指摘は理由を添えてスキップしてよい。
- `.coderabbit.yaml` はベースブランチ（main）の内容が適用される。設定変更はマージして初めて有効になる。

## マージ

CI グリーン & レビュー解決後、**squash マージ + ブランチ削除**で main をクリーンに保つ：

```sh
gh pr merge <n> --squash --delete-branch
```

## クイックリファレンス

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
