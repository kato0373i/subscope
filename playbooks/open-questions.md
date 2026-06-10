# playbook: 検討事項・実装前チェックリスト（状態付き）

新規実装の前に、関連するものはここを意識する。状態は [docs/domain-design.md](../docs/domain-design.md) §7 が正典で、ここはその索引。

## 決定済み（実装あり）

- **CreditNote（赤伝/返金）** — ✅ 決定（#18）。独立集約 `creditnote.CreditNote`（`issued → applied` の状態機械、`ContractID` 紐付け）として実装済み。`Money.IsNegative` は返金判定用。詳細は §7。
- **締め日** — ✅ 決定（#18）。`contract` ドメインに配置し、`ClosingPolicy`（締め日 VO・月末締め対応の `ClosingDate`）として実装済み。詳細は §7。

## 未決定（実装前に要検討）

- **冪等性** — PSP の Webhook 二重通知に備え、`payment` / `settlement` には冪等キーが要る。
- **Outbox パターン** — 現状のイベントバスはインメモリ・同期実装（`platform/eventbus`）。本番ではトランザクション境界を守るため Outbox + メッセージング基盤に差し替える。
- **永続化** — 現状は in-memory map。PostgreSQL + マイグレーション + モジュール別スキーマへ。
