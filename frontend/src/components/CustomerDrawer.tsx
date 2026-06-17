import { useEffect, useState } from "react";
import type { SubscopeApi } from "../api/client";
import type { CustomerDetail } from "../api/types";
import {
  collectionStatusLabel,
  collectionStatusTone,
  contractStatusLabel,
  contractStatusTone,
  formatMoney,
} from "../format";
import { StatusPill } from "./StatusPill";

interface Props {
  api: SubscopeApi;
  /** 表示中の契約 ID。null ならドロワーは閉じている。 */
  contractId: string | null;
  onClose: () => void;
}

/**
 * 顧客個票（顧客360）の右スライドドロワー。
 * contractId が変わるたびに GET /api/contracts/{id} を取得し、
 * 契約ヘッダ・サマリ・請求履歴（回収ステータス付き）を 1 画面で表示する。
 */
export function CustomerDrawer({ api, contractId, onClose }: Props) {
  const [detail, setDetail] = useState<CustomerDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!contractId) return;
    let active = true;
    setLoading(true);
    setError(null);
    setDetail(null);
    api
      .getCustomerDetail(contractId)
      .then((d) => {
        if (active) setDetail(d);
      })
      .catch((err: unknown) => {
        if (active) {
          setError(err instanceof Error ? err.message : "個票の取得に失敗しました");
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [api, contractId]);

  // Esc キーで閉じる。
  useEffect(() => {
    if (!contractId) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [contractId, onClose]);

  if (!contractId) return null;

  return (
    <div className="drawer">
      <div className="drawer__backdrop" onClick={onClose} />
      <aside
        className="drawer__panel"
        role="dialog"
        aria-modal="true"
        aria-label="顧客個票"
      >
        <header className="drawer__head">
          <div>
            <span className="drawer__eyebrow">顧客個票</span>
            <h2 className="drawer__title mono">{contractId}</h2>
          </div>
          <button
            type="button"
            className="btn btn--sm btn--ghost"
            onClick={onClose}
            aria-label="閉じる"
          >
            閉じる
          </button>
        </header>

        <div className="drawer__body">
          {loading ? (
            <div className="state">読み込み中…</div>
          ) : error ? (
            <div className="state state--error">エラー: {error}</div>
          ) : detail ? (
            <>
              <section className="drawer__section">
                <div className="drawer__contract">
                  <div className="drawer__member">{detail.contract.memberName}</div>
                  <StatusPill
                    label={contractStatusLabel(detail.contract.status)}
                    tone={contractStatusTone(detail.contract.status)}
                  />
                </div>
                <dl className="drawer__facts">
                  <div>
                    <dt>請求先</dt>
                    <dd className="mono muted">{detail.contract.billingAccountId}</dd>
                  </div>
                  <div>
                    <dt>月会費</dt>
                    <dd className="num">{formatMoney(detail.contract.monthlyFee)}</dd>
                  </div>
                </dl>
              </section>

              <section className="drawer__summary">
                <div className="drawer__stat">
                  <span className="drawer__stat-label">入金済み</span>
                  <span className="drawer__stat-value">
                    {formatMoney(detail.summary.paid)}
                  </span>
                </div>
                <div className="drawer__stat">
                  <span className="drawer__stat-label">債権残</span>
                  <span className="drawer__stat-value">
                    {formatMoney(detail.summary.outstanding)}
                  </span>
                </div>
                <div className="drawer__stat">
                  <span className="drawer__stat-label">回収中</span>
                  <span className="drawer__stat-value">
                    {detail.summary.inCollection} 件
                  </span>
                </div>
              </section>

              <section className="drawer__section">
                <h3 className="drawer__subtitle">
                  請求履歴
                  <span className="card__count">{detail.summary.invoiceCount} 件</span>
                </h3>
                <div className="table-wrap">
                  <table className="table">
                    <thead>
                      <tr>
                        <th>請求ID</th>
                        <th className="ar">金額</th>
                        <th>回収状況</th>
                      </tr>
                    </thead>
                    <tbody>
                      {detail.invoices.length === 0 ? (
                        <tr className="empty-row">
                          <td colSpan={3}>請求がありません。</td>
                        </tr>
                      ) : (
                        detail.invoices.map((row) => (
                          <tr key={row.invoiceId}>
                            <td className="mono">{row.invoiceId}</td>
                            <td className="ar num">{formatMoney(row.amount)}</td>
                            <td>
                              <StatusPill
                                label={collectionStatusLabel(row.collectionStatus)}
                                tone={collectionStatusTone(row.collectionStatus)}
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
          ) : null}
        </div>
      </aside>
    </div>
  );
}
