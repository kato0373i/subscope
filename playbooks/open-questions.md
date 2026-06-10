# playbook: 検討事項・未着手の論点

新規実装の前に、関連するものはここを意識する（詳細は [docs/domain-design.md](../docs/domain-design.md) §7）。

- **CreditNote（赤伝/返金）** — インボイス制度的には適格返還請求書として別文書が推奨。`Money.IsNegative` は返金判定用に用意済み。
- **冪等性** — PSP の Webhook 二重通知に備え、`payment` / `settlement` には冪等キーが要る。
- **Outbox パターン** — 現状のイベントバスはインメモリ・同期実装（`platform/eventbus`）。本番ではトランザクション境界を守るため Outbox + メッセージング基盤に差し替える。
- **締め日** — 月末締め翌月請求のような締め概念を `contract` / `billing` のどちらに置くか。
- **永続化** — 現状は in-memory map。PostgreSQL + マイグレーション + モジュール別スキーマへ。
