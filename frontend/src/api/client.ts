import type { Contract, CollectionState } from "./types";

// 接続方針（#20 / #35）:
// フロントは「SubscopeApi インターフェース」越しにデータを取得し、UI は API 抽象にのみ依存する。
// 実装は MockApi（決定的サンプル）と HttpApi（REST API: internal/platform/httpapi）の2つ。
// VITE_API_BASE_URL が設定されていれば HttpApi、未設定なら MockApi を既定にする。

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

/**
 * HttpApi はバックエンドの REST API（internal/platform/httpapi）に接続する実装。
 * DTO は types.ts と 1:1 整合しているため、レスポンスをそのまま返す。
 */
export class HttpApi implements SubscopeApi {
  constructor(private readonly baseUrl: string) {}

  async listContracts(): Promise<Contract[]> {
    return this.get<Contract[]>("/api/contracts");
  }

  async listCollectionStates(): Promise<CollectionState[]> {
    return this.get<CollectionState[]>("/api/collection-states");
  }

  private async get<T>(path: string): Promise<T> {
    const res = await fetch(`${this.baseUrl}${path}`);
    if (!res.ok) {
      throw new Error(`API ${path} が失敗しました: ${res.status}`);
    }
    return res.json() as Promise<T>;
  }
}

/**
 * 既定の API 実装。VITE_API_BASE_URL が設定されていれば実 API（HttpApi）、
 * 未設定なら MockApi を用いる（当面は安全側でモック既定）。
 */
const baseUrl = import.meta.env.VITE_API_BASE_URL;
export const api: SubscopeApi = baseUrl
  ? new HttpApi(baseUrl)
  : new MockApi();
