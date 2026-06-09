import type { Contract, CollectionState } from "./types";

// 接続方針（#20 の decision）:
// バックエンドは現状 HTTP API を持たず、cmd/api のデモ実行のみ。
// そこでフロントは「SubscopeApi インターフェース」越しにデータを取得する形にし、
// 既定実装はモック（MockApi）とする。バックエンドに REST/HTTP 層が入った段階で
// HttpApi 実装を追加して差し替えるだけで済むよう、UI は API 抽象にのみ依存する。

/** フロントが依存するデータ取得の境界。実装はモック／HTTP を差し替え可能。 */
export interface SubscopeApi {
  listContracts(): Promise<Contract[]>;
  listCollectionStates(): Promise<CollectionState[]>;
}

const jpy = (amount: number) => ({ amount, currency: "JPY" });

const mockContracts: Contract[] = [
  {
    id: "CT-0001",
    memberName: "山田 太郎",
    billingAccountId: "BA-0001",
    monthlyFee: jpy(3000),
    status: "active",
  },
  {
    id: "CT-0002",
    memberName: "佐藤 花子",
    billingAccountId: "BA-0002",
    monthlyFee: jpy(5000),
    status: "trialing",
  },
  {
    id: "CT-0003",
    memberName: "鈴木 一郎",
    billingAccountId: "BA-0003",
    monthlyFee: jpy(3000),
    status: "past_due",
  },
  {
    id: "CT-0004",
    memberName: "高橋 美咲",
    billingAccountId: "BA-0004",
    monthlyFee: jpy(8000),
    status: "active",
  },
  {
    id: "CT-0005",
    memberName: "田中 健",
    billingAccountId: "BA-0005",
    monthlyFee: jpy(3000),
    status: "suspended",
  },
  {
    id: "CT-0006",
    memberName: "渡辺 さくら",
    billingAccountId: "BA-0006",
    monthlyFee: jpy(12000),
    status: "active",
  },
];

const mockCollectionStates: CollectionState[] = [
  {
    invoiceId: "INV-0001",
    contractId: "CT-0001",
    amount: jpy(3000),
    status: "paid",
  },
  {
    invoiceId: "INV-0002",
    contractId: "CT-0003",
    amount: jpy(3000),
    status: "in_collection",
  },
  {
    invoiceId: "INV-0003",
    contractId: "CT-0002",
    amount: jpy(5000),
    status: "issued",
  },
  {
    invoiceId: "INV-0004",
    contractId: "CT-0004",
    amount: jpy(8000),
    status: "paid",
  },
  {
    invoiceId: "INV-0005",
    contractId: "CT-0006",
    amount: jpy(12000),
    status: "partially_paid",
  },
  {
    invoiceId: "INV-0006",
    contractId: "CT-0005",
    amount: jpy(3000),
    status: "written_off",
  },
];

/** MockApi はバックエンド API 整備前の暫定データ源。決定的なサンプルを返す。 */
export class MockApi implements SubscopeApi {
  listContracts(): Promise<Contract[]> {
    return Promise.resolve(mockContracts);
  }

  listCollectionStates(): Promise<CollectionState[]> {
    return Promise.resolve(mockCollectionStates);
  }
}

/** 既定の API 実装。HTTP 層が入るまではモックを用いる。 */
export const api: SubscopeApi = new MockApi();
