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

/** 請求書 1 行 + 回収ステータス（顧客個票用）。GET /api/contracts/{id} の invoices[]。 */
export interface InvoiceCollectionRow {
  invoiceId: string;
  amount: Money;
  /** billing 由来の生ステータス（issued / paid …）。 */
  invoiceStatus: string;
  /** billing×collection を合成した画面用ステータス。 */
  collectionStatus: CollectionStatus;
}

/** 顧客個票（顧客360）。GET /api/contracts/{id} に対応。 */
export interface CustomerDetail {
  contract: Contract;
  invoices: InvoiceCollectionRow[];
  summary: {
    invoiceCount: number;
    /** 入金済み合計。 */
    paid: Money;
    /** 未入金合計（債権残）。 */
    outstanding: Money;
    /** 回収中の件数。 */
    inCollection: number;
  };
}

/** 督促キャンペーンの状態。バックエンド dunning の状態機械に対応。 */
export type DunningStatus = "active" | "resolved" | "completed";

/** 督促キャンペーンの 1 行。GET /api/dunning-campaigns に対応。 */
export interface DunningCampaign {
  campaignId: string;
  invoiceId: string;
  account: string;
  status: DunningStatus;
  /** 実施済みステップ数。 */
  stepsTriggered: number;
  /** 全ステップ数。 */
  stepsTotal: number;
  /** 次に発火するチャネル（email/sms/letter）。完了なら ""。 */
  nextChannel: string;
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
