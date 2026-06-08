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
