import type { Money } from "./api/types";

/** Money を「¥3,000」のような日本円表記に整形する。 */
export function formatMoney(money: Money): string {
  if (money.currency === "JPY") {
    return `¥${money.amount.toLocaleString("ja-JP")}`;
  }
  return `${money.amount.toLocaleString()} ${money.currency}`;
}

/** ステータスピルの色調。Stripe 風の意味色に対応する。 */
export type Tone = "positive" | "warning" | "critical" | "info" | "neutral";

const contractStatusLabels: Record<string, string> = {
  trialing: "トライアル中",
  active: "有効",
  past_due: "支払遅延",
  suspended: "利用停止",
  cancelled: "解約",
};

const contractStatusTones: Record<string, Tone> = {
  trialing: "info",
  active: "positive",
  past_due: "critical",
  suspended: "warning",
  cancelled: "neutral",
};

const collectionStatusLabels: Record<string, string> = {
  issued: "請求済み",
  paid: "入金済み",
  partially_paid: "一部入金",
  in_collection: "回収中",
  written_off: "貸倒",
};

const collectionStatusTones: Record<string, Tone> = {
  issued: "info",
  paid: "positive",
  partially_paid: "warning",
  in_collection: "critical",
  written_off: "neutral",
};

export const contractStatusLabel = (status: string): string =>
  contractStatusLabels[status] ?? status;

export const contractStatusTone = (status: string): Tone =>
  contractStatusTones[status] ?? "neutral";

export const collectionStatusLabel = (status: string): string =>
  collectionStatusLabels[status] ?? status;

export const collectionStatusTone = (status: string): Tone =>
  collectionStatusTones[status] ?? "neutral";
