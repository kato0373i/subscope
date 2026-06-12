// バックエンドのドメイン（contract / billing / collection）に対応する DTO 型。
// バックエンドの shared.Money・各 ID と 1:1 で対応させ、API 境界の契約をここに固定する。

/** 金額。バックエンドの shared.Money（最小通貨単位=円, int64）に対応。 */
export interface Money {
  amount: number;
  currency: string;
}

/** 契約の状態。バックエンド contract の状態機械に対応。 */
export type ContractStatus =
  | "trialing"
  | "active"
  | "past_due"
  | "suspended"
  | "cancelled";

/** 請求〜回収の状況。billing/collection の進捗を画面表示用に集約したビュー。 */
export type CollectionStatus =
  | "issued"
  | "paid"
  | "partially_paid"
  | "in_collection"
  | "written_off";

/** 契約一覧の 1 行。 */
export interface Contract {
  id: string;
  memberName: string;
  billingAccountId: string;
  monthlyFee: Money;
  status: ContractStatus;
}

/** 請求/回収状況の 1 行。 */
export interface CollectionState {
  invoiceId: string;
  contractId: string;
  amount: Money;
  status: CollectionStatus;
}

// --- 操作（コマンド）系の入出力。バックエンド httpapi の REST 契約に対応。 ---

/** 契約登録の入力（POST /api/contracts）。 */
export interface RegisterContractInput {
  id: string;
  memberId: string;
  billingAccountId: string;
  monthlyFee: Money;
}

/** Billing Run（定期請求の自動起票）の入力（POST /api/billing-runs、全フィールド任意）。 */
export interface BillingRunInput {
  /** YYYY-MM-DD。省略時はサーバの現在時刻。 */
  asOf?: string;
  /** true なら抽出のみ（起票しない）。 */
  dryRun?: boolean;
}

/** Billing Run が起票（予定）した 1 件。決済手段は持たない（債権≠決済手段）。 */
export interface BillingRunItem {
  contractId: string;
  billingAccountId: string;
  amount: Money;
  period: string;
}

/** Billing Run の実行結果。 */
export interface BillingRunResult {
  runId: string;
  asOf: string;
  dryRun: boolean;
  items: BillingRunItem[];
  skipped: number;
}
