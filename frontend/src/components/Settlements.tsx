import { useCallback, useEffect, useState, type FormEvent } from "react";
import type { SubscopeApi } from "../api/client";
import type { OutstandingInvoice, Settlement } from "../api/types";
import { formatMoney } from "../format";
import { StatusPill } from "./StatusPill";

type Notify = (message: string, kind: "success" | "error") => void;

/** 数値入力を 0 以上の整数に正規化する（空/途中編集で NaN・負数を state へ入れない）。 */
const toPositiveInt = (raw: string): number => {
  const n = Number(raw);
  if (!Number.isFinite(n)) return 0;
  return Math.max(0, Math.trunc(n));
};

interface Props {
  api: SubscopeApi;
  notify: Notify;
  /** 消込で billing/collection 側の表示も変わるため、全体の再取得を促す。 */
  refresh: () => Promise<void>;
}

/**
 * 入金・消込画面。未消込（消込候補）と消込実績を 2 ペインで表示し、
 * 銀行入金取込と手動消込のコマンドを実行する。
 * 「まだ入っていない（pending な債権）」と「入った（確定）」を画面で峻別する（入金は非同期）。
 */
export function Settlements({ api, notify, refresh }: Props) {
  const [settlements, setSettlements] = useState<Settlement[]>([]);
  const [outstanding, setOutstanding] = useState<OutstandingInvoice[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    const [s, o] = await Promise.all([
      api.listSettlements(),
      api.listOutstanding(),
    ]);
    setSettlements(s);
    setOutstanding(o);
  }, [api]);

  useEffect(() => {
    let active = true;
    reload()
      .catch((err: unknown) => {
        if (active) {
          setError(err instanceof Error ? err.message : "取得に失敗しました");
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [reload]);

  /** 消込/取込の後、自身と全体の両方を再取得する。 */
  const afterCommand = useCallback(async () => {
    await Promise.all([reload(), refresh()]);
  }, [reload, refresh]);

  if (loading) return <div className="state">読み込み中…</div>;
  if (error) return <div className="state state--error">エラー: {error}</div>;

  return (
    <>
      <DepositImportForm api={api} notify={notify} onDone={afterCommand} />

      <section className="card">
        <div className="card__head">
          <h2 className="card__title">未消込（消込候補）</h2>
          <span className="card__count">{outstanding.length} 件</span>
        </div>
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>請求ID</th>
                <th>請求先</th>
                <th>名義</th>
                <th className="ar">残額</th>
                <th className="ar">手動消込</th>
              </tr>
            </thead>
            <tbody>
              {outstanding.length === 0 ? (
                <tr className="empty-row">
                  <td colSpan={5}>未消込の請求はありません。</td>
                </tr>
              ) : (
                outstanding.map((o) => (
                  <OutstandingRow
                    key={o.invoiceId}
                    api={api}
                    notify={notify}
                    row={o}
                    onDone={afterCommand}
                  />
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>

      <section className="card">
        <div className="card__head">
          <h2 className="card__title">消込実績</h2>
          <span className="card__count">{settlements.length} 件</span>
        </div>
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>消込ID</th>
                <th>請求ID</th>
                <th className="ar">入金額</th>
                <th className="ar">充当済み</th>
                <th>充当</th>
              </tr>
            </thead>
            <tbody>
              {settlements.length === 0 ? (
                <tr className="empty-row">
                  <td colSpan={5}>消込実績はありません。</td>
                </tr>
              ) : (
                settlements.map((s) => (
                  <tr key={s.settlementId}>
                    <td className="mono">{s.settlementId}</td>
                    <td className="mono muted">{s.invoiceId}</td>
                    <td className="ar num">{formatMoney(s.amount)}</td>
                    <td className="ar num">{formatMoney(s.reconciled)}</td>
                    <td>
                      <StatusPill
                        label={s.fullyApplied ? "全額" : "一部"}
                        tone={s.fullyApplied ? "positive" : "warning"}
                      />
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>
    </>
  );
}

/** 未消込 1 行 + 手動消込フォーム（残額を初期値に充当額を入力）。 */
function OutstandingRow({
  api,
  notify,
  row,
  onDone,
}: {
  api: SubscopeApi;
  notify: Notify;
  row: OutstandingInvoice;
  onDone: () => Promise<void>;
}) {
  const [amount, setAmount] = useState(row.outstanding.amount);
  const [busy, setBusy] = useState(false);

  // 再取得で同一 invoiceId の行が残り key 再利用されたとき、入力を最新の残額へ同期する。
  useEffect(() => {
    setAmount(row.outstanding.amount);
  }, [row.invoiceId, row.outstanding.amount]);

  const reconcile = async () => {
    if (!Number.isInteger(amount) || amount <= 0) {
      notify("充当額は 1 以上の整数で入力してください", "error");
      return;
    }
    setBusy(true);
    try {
      await api.reconcileManually({
        invoiceId: row.invoiceId,
        amount: { amount, currency: row.outstanding.currency },
      });
      await onDone();
      notify(`${row.invoiceId} を ${formatMoney({ amount, currency: row.outstanding.currency })} 消し込みました`, "success");
    } catch (err) {
      notify(err instanceof Error ? err.message : "消込に失敗しました", "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <tr>
      <td className="mono">{row.invoiceId}</td>
      <td className="mono muted">{row.account}</td>
      <td>{row.payerName || "—"}</td>
      <td className="ar num">{formatMoney(row.outstanding)}</td>
      <td className="ar">
        <div className="inline-form">
          <input
            className="input input--sm"
            type="number"
            min={1}
            step={1}
            value={amount}
            onChange={(e) => setAmount(toPositiveInt(e.target.value))}
            aria-label="充当額（円）"
          />
          <button
            type="button"
            className="btn btn--sm"
            disabled={busy}
            onClick={reconcile}
          >
            {busy ? "消込中…" : "消込"}
          </button>
        </div>
      </td>
    </tr>
  );
}

/** 銀行入金取込フォーム（1 件単位の MVP）。POST /api/bank-deposits。 */
function DepositImportForm({
  api,
  notify,
  onDone,
}: {
  api: SubscopeApi;
  notify: Notify;
  onDone: () => Promise<void>;
}) {
  const [reference, setReference] = useState("");
  const [account, setAccount] = useState("BA-0001");
  const [payerName, setPayerName] = useState("");
  const [amount, setAmount] = useState(3000);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!Number.isInteger(amount) || amount <= 0) {
      setError("入金額は 1 以上の整数で入力してください");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const res = await api.importBankDeposits([
        { reference, account, payerName, amount: { amount, currency: "JPY" } },
      ]);
      await onDone();
      notify(`入金を ${res.imported} 件取り込みました（自動照合 → 未照合は手動消込へ）`, "success");
      setReference("");
      setPayerName("");
    } catch (err) {
      const m = err instanceof Error ? err.message : "取込に失敗しました";
      setError(m);
      notify(m, "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="card">
      <div className="card__head">
        <div className="section-title">
          <span className="section-title__text">銀行入金取込</span>
          <span className="section-title__sub">
            入金を取り込み、請求先ID・名義＋金額で自動照合する（未照合は下の手動消込へ）
          </span>
        </div>
      </div>
      <div className="card__body">
        <form onSubmit={onSubmit}>
          <div className="form-grid">
            <label className="field">
              <span className="field__label">入金参照番号</span>
              <input
                className="input"
                value={reference}
                onChange={(e) => setReference(e.target.value)}
                placeholder="R-1001（冪等キー）"
                required
              />
            </label>
            <label className="field">
              <span className="field__label">請求先ID</span>
              <input
                className="input"
                value={account}
                onChange={(e) => setAccount(e.target.value)}
                placeholder="BA-0001"
              />
            </label>
            <label className="field">
              <span className="field__label">振込人名義</span>
              <input
                className="input"
                value={payerName}
                onChange={(e) => setPayerName(e.target.value)}
                placeholder="ヤマダ タロウ"
              />
            </label>
            <label className="field">
              <span className="field__label">入金額（円）</span>
              <input
                className="input"
                type="number"
                min={1}
                step={1}
                value={amount}
                onChange={(e) => setAmount(toPositiveInt(e.target.value))}
              />
            </label>
          </div>
          <div className="form-foot">
            <button type="submit" className="btn" disabled={busy}>
              {busy ? "取込中…" : "入金を取り込む"}
            </button>
            {error && <span className="form-error">{error}</span>}
          </div>
        </form>
      </div>
    </section>
  );
}
