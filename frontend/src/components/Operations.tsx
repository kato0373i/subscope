import { useState, type FormEvent, type ReactNode } from "react";
import type { SubscopeApi } from "../api/client";
import type { BillingRunResult } from "../api/types";
import { formatMoney } from "../format";

type Notify = (message: string, kind: "success" | "error") => void;

interface Props {
  api: SubscopeApi;
  notify: Notify;
  refresh: () => Promise<void>;
}

/**
 * 登録・操作画面。バックエンド httpapi が公開するコマンド（契約登録・請求実行・Billing Run）を
 * UI から実行し、結果を一覧に反映する。契約一覧の「請求実行」と合わせて一連の操作性をつなぐ。
 */
export function Operations({ api, notify, refresh }: Props) {
  return (
    <>
      <Section
        step={1}
        title="契約を登録"
        sub="会員・請求先・月会費を指定して契約を起こす（債権は決済手段を持たない）"
      >
        <ContractForm api={api} notify={notify} refresh={refresh} />
      </Section>

      <Section
        step={2}
        title="Billing Run（定期請求の自動起票）"
        sub="対象日時点で請求サイクルが到来した契約を抽出して一括起票する"
      >
        <BillingRunForm api={api} notify={notify} refresh={refresh} />
      </Section>
    </>
  );
}

/** 番号付きの操作セクション枠。 */
function Section({
  step,
  title,
  sub,
  children,
}: {
  step: number;
  title: string;
  sub: string;
  children: ReactNode;
}) {
  return (
    <section className="card">
      <div className="card__head">
        <div className="section-title">
          <span className="section-title__step">{step}</span>
          <span className="section-title__text">{title}</span>
          <span className="section-title__sub">{sub}</span>
        </div>
      </div>
      <div className="card__body">{children}</div>
    </section>
  );
}

/** ラベル付きフォーム項目。 */
function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <label className="field">
      <span className="field__label">{label}</span>
      {children}
      {hint && <span className="field__hint">{hint}</span>}
    </label>
  );
}

/** 送信ボタンとエラー表示をまとめたフッタ。 */
function Foot({
  busy,
  error,
  label,
}: {
  busy: boolean;
  error: string | null;
  label: string;
}) {
  return (
    <div className="form-foot">
      <button type="submit" className="btn" disabled={busy}>
        {busy ? "送信中…" : label}
      </button>
      {error && <span className="form-error">{error}</span>}
    </div>
  );
}

/** 契約登録フォーム（POST /api/contracts）。 */
function ContractForm({ api, notify, refresh }: Props) {
  const [id, setId] = useState("");
  const [memberId, setMemberId] = useState("MEM-0001");
  const [billingAccountId, setBillingAccountId] = useState("BA-0001");
  const [monthlyFee, setMonthlyFee] = useState(3000);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const res = await api.registerContract({
        id,
        memberId,
        billingAccountId,
        monthlyFee: { amount: monthlyFee, currency: "JPY" },
      });
      await refresh();
      notify(`契約 ${res.id} を登録しました。契約一覧から請求を実行できます`, "success");
      setId("");
    } catch (err) {
      const m = err instanceof Error ? err.message : "登録に失敗しました";
      setError(m);
      notify(m, "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={onSubmit}>
      <div className="form-grid">
        <Field label="契約ID" hint="例: CT-1001（一意）">
          <input
            className="input"
            value={id}
            onChange={(e) => setId(e.target.value)}
            placeholder="CT-1001"
            required
          />
        </Field>
        <Field label="会員ID" hint="シードに MEM-0001 あり">
          <input
            className="input"
            value={memberId}
            onChange={(e) => setMemberId(e.target.value)}
            placeholder="MEM-0001"
            required
          />
        </Field>
        <Field label="請求先ID" hint="シードに BA-0001 あり">
          <input
            className="input"
            value={billingAccountId}
            onChange={(e) => setBillingAccountId(e.target.value)}
            placeholder="BA-0001"
            required
          />
        </Field>
        <Field label="月会費（円）">
          <input
            className="input"
            type="number"
            min={1}
            value={monthlyFee}
            onChange={(e) => setMonthlyFee(Number(e.target.value))}
          />
        </Field>
      </div>
      <Foot busy={busy} error={error} label="契約を登録" />
    </form>
  );
}

/** Billing Run フォーム（POST /api/billing-runs）。ドライランで抽出プレビュー、本実行で起票。 */
function BillingRunForm({ api, notify, refresh }: Props) {
  const [asOf, setAsOf] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [preview, setPreview] = useState<BillingRunResult | null>(null);

  const run = async (dryRun: boolean) => {
    setBusy(true);
    setError(null);
    try {
      const result = await api.runBilling({ asOf: asOf || undefined, dryRun });
      setPreview(result);
      if (dryRun) {
        notify(`ドライラン: 対象 ${result.items.length} 件 / スキップ ${result.skipped} 件`, "success");
      } else {
        await refresh();
        notify(`Billing Run 実行: ${result.items.length} 件を起票しました`, "success");
      }
    } catch (err) {
      const m = err instanceof Error ? err.message : "実行に失敗しました";
      setError(m);
      notify(m, "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={(e) => e.preventDefault()}>
      <div className="form-grid">
        <Field label="対象日（asOf）" hint="空欄ならサーバの現在時刻">
          <input
            className="input"
            type="date"
            value={asOf}
            onChange={(e) => setAsOf(e.target.value)}
          />
        </Field>
      </div>
      <div className="form-foot">
        <button type="button" className="btn btn--ghost" disabled={busy} onClick={() => run(true)}>
          {busy ? "実行中…" : "ドライラン"}
        </button>
        <button type="button" className="btn" disabled={busy} onClick={() => run(false)}>
          {busy ? "実行中…" : "本実行"}
        </button>
        {error && <span className="form-error">{error}</span>}
      </div>

      {preview && (
        <div className="run-result">
          <div className="run-result__head">
            実行 {preview.runId || "(dry)"} ・ asOf {preview.asOf} ・{" "}
            {preview.dryRun ? "ドライラン" : "本実行"} ・ 対象 {preview.items.length} 件 / スキップ{" "}
            {preview.skipped} 件
          </div>
          {preview.items.length > 0 && (
            <ul className="run-result__list">
              {preview.items.map((it) => (
                <li key={`${it.contractId}-${it.period}`}>
                  <span className="mono">{it.contractId}</span> ・ {it.period} ・{" "}
                  {formatMoney(it.amount)}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </form>
  );
}
