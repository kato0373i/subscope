import type {
  BillingRunInput,
  BillingRunResult,
  Contract,
  CollectionState,
  RegisterContractInput,
} from "./types";

// 接続方針（#20 / #35 / #59）:
// フロントは「SubscopeApi インターフェース」越しにデータ取得・操作を行い、UI は API 抽象にのみ依存する。
// 実装は MockApi（決定的サンプル・読み取り専用）と HttpApi（REST API: internal/platform/httpapi）の2つ。
// VITE_API_BASE_URL が定義されていれば HttpApi（空文字 "" は同一オリジン）、未定義なら MockApi を既定にする。
// Docker では同一プロセスが UI と API を配信するため、ビルド時に VITE_API_BASE_URL="" を渡して HttpApi にする。

/** フロントが依存するデータ取得・操作の境界。実装はモック／HTTP を差し替え可能。 */
export interface SubscopeApi {
  listContracts(): Promise<Contract[]>;
  listCollectionStates(): Promise<CollectionState[]>;
  registerContract(input: RegisterContractInput): Promise<{ id: string }>;
  triggerBilling(contractId: string): Promise<void>;
  runBilling(input: BillingRunInput): Promise<BillingRunResult>;
}

/** モックは読み取り専用。操作系は実 API（HttpApi）でのみ利用できる。 */
const notSupported = (): never => {
  throw new Error("モック環境では操作 API は利用できません（VITE_API_BASE_URL を設定してください）");
};

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

/** MockApi はバックエンド API 整備前の暫定データ源。決定的なサンプルを返す（読み取り専用）。 */
export class MockApi implements SubscopeApi {
  /** 契約一覧のサンプルを返す。 */
  listContracts(): Promise<Contract[]> {
    return Promise.resolve(mockContracts);
  }

  /** 請求/回収状況のサンプルを返す。 */
  listCollectionStates(): Promise<CollectionState[]> {
    return Promise.resolve(mockCollectionStates);
  }

  /** 操作系はモックでは未対応（実 API でのみ利用可）。 */
  registerContract(): Promise<{ id: string }> {
    return Promise.resolve(notSupported());
  }

  /** 操作系はモックでは未対応（実 API でのみ利用可）。 */
  triggerBilling(): Promise<void> {
    return Promise.resolve(notSupported());
  }

  /** 操作系はモックでは未対応（実 API でのみ利用可）。 */
  runBilling(): Promise<BillingRunResult> {
    return Promise.resolve(notSupported());
  }
}

/**
 * HttpApi はバックエンドの REST API（internal/platform/httpapi）に接続する実装。
 * DTO は types.ts と 1:1 整合しているため、レスポンスをそのまま返す。
 */
export class HttpApi implements SubscopeApi {
  /** baseUrl は API のベース URL。空文字なら同一オリジン（相対パス）を叩く。 */
  constructor(private readonly baseUrl: string) {}

  /** 契約一覧を取得する（GET /api/contracts）。 */
  async listContracts(): Promise<Contract[]> {
    return this.get<Contract[]>("/api/contracts");
  }

  /** 請求/回収状況を取得する（GET /api/collection-states）。 */
  async listCollectionStates(): Promise<CollectionState[]> {
    return this.get<CollectionState[]>("/api/collection-states");
  }

  /** 契約を登録する（POST /api/contracts）。 */
  async registerContract(input: RegisterContractInput): Promise<{ id: string }> {
    return this.post<{ id: string }>("/api/contracts", input);
  }

  /** 単一契約の請求サイクルを起動する（POST /api/contracts/{id}/billing）。 */
  async triggerBilling(contractId: string): Promise<void> {
    await this.post<unknown>(`/api/contracts/${contractId}/billing`, {});
  }

  /** Billing Run（定期請求の自動起票）を実行する（POST /api/billing-runs）。 */
  async runBilling(input: BillingRunInput): Promise<BillingRunResult> {
    return this.post<BillingRunResult>("/api/billing-runs", input);
  }

  /** GET リクエストを送り JSON を返す共通ヘルパー。 */
  private async get<T>(path: string): Promise<T> {
    const res = await fetch(`${this.baseUrl}${path}`);
    if (!res.ok) {
      throw new Error(`API ${path} が失敗しました: ${res.status}`);
    }
    return res.json() as Promise<T>;
  }

  /** POST リクエストを送り、エラー時は本文 { error: { message } } を拾って投げる共通ヘルパー。 */
  private async post<T>(path: string, body: unknown): Promise<T> {
    const res = await fetch(`${this.baseUrl}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      // エラー本文 { error: { code, message } } を可能なら取り出す。
      let message = `${res.status}`;
      try {
        const data = (await res.json()) as { error?: { message?: string } };
        if (data.error?.message) message = data.error.message;
      } catch {
        // JSON でなければステータスのみ。
      }
      throw new Error(`API ${path} が失敗しました: ${message}`);
    }
    const text = await res.text();
    return (text ? JSON.parse(text) : undefined) as T;
  }
}

/**
 * 既定の API 実装。VITE_API_BASE_URL が「定義されていれば」実 API（HttpApi）を使う。
 * 空文字 "" は同一オリジン（相対パス /api/...）を意味し、Docker の単一プロセス配信で用いる。
 * 環境変数が未定義のときだけ MockApi にフォールバックする（バックエンド無しの開発時）。
 */
const baseUrl = import.meta.env.VITE_API_BASE_URL;
export const api: SubscopeApi =
  baseUrl === undefined ? new MockApi() : new HttpApi(baseUrl);
