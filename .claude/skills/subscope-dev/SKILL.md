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

規約と手順は、コンテキスト/キャッシュ節約のため**ルート直下に移設済み**。重複を避けるため、ここには索引だけを置く。

## 常時必要なルール

→ [`/CLAUDE.md`](../../../CLAUDE.md)（設計の背骨3原則、絶対ルール、モデルルーティング、サブエージェント委譲、コード検索＝semble 優先、キャッシュ運用）

## 作業別の手順（playbooks）

作業に入るときだけ該当ファイルを読む。

| いつ読むか | playbook |
|---|---|
| モジュール/集約/イベントを追加・変更する、境界の型を確認する | [`/playbooks/module-design.md`](../../../playbooks/module-design.md) |
| ブランチ・コミット・PR・CI・CodeRabbit 対応・マージ | [`/playbooks/git-workflow.md`](../../../playbooks/git-workflow.md) |
| コード検索の方針、semble のセットアップ・使い方 | [`/playbooks/code-search.md`](../../../playbooks/code-search.md) |
| 新規実装前に踏まえる未着手の論点 | [`/playbooks/open-questions.md`](../../../playbooks/open-questions.md) |

ドメインモデルの正典は [docs/domain-design.md](../../../docs/domain-design.md)。参照実装：境界の作り方は `internal/tax`、イベント駆動の回収戦略は `internal/collection`。
