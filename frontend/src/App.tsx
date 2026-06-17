import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "./api/client";
import type { CollectionState, Contract, DunningCampaign } from "./api/types";
import {
  collectionStatusLabel,
  collectionStatusTone,
  contractStatusLabel,
  contractStatusTone,
  dunningChannelLabel,
  dunningStatusLabel,
  dunningStatusTone,
  formatMoney,
} from "./format";
import type { View } from "./views";
import { Sidebar } from "./components/Sidebar";
import { Operations } from "./components/Operations";
import { Settlements } from "./components/Settlements";
import { MetricCard } from "./components/MetricCard";
import { StatusPill } from "./components/StatusPill";
import { CustomerDrawer } from "./components/CustomerDrawer";
import { IconAlert, IconCheck, IconRevenue, IconUsers } from "./components/icons";
import "./App.css";

const pageMeta: Record<View, { title: string; subtitle: string }> = {
  dashboard: { title: "ダッシュボード", subtitle: "サブスクリプション業務の概況" },
  operations: { title: "登録・操作", subtitle: "契約登録・請求実行・Billing Run" },
  contracts: { title: "契約", subtitle: "契約の一覧と状態・請求実行" },
  collections: { title: "請求・回収", subtitle: "請求書ごとの入金・回収状況" },
  settlement: { title: "入金・消込", subtitle: "銀行入金の取込・自動照合・手動消込" },
  dunning: { title: "督促", subtitle: "未収に対する督促キャンペーンの進行状況" },
};

interface Toast {
  message: string;
  kind: "success" | "error";
}

/** 管理画面のルート。ダッシュボード・登録/操作・契約・請求/回収の各ビューを束ねる。 */
function App() {
  const [view, setView] = useState<View>("dashboard");
  const [contracts, setContracts] = useState<Contract[]>([]);
  const [collections, setCollections] = useState<CollectionState[]>([]);
  const [dunning, setDunning] = useState<DunningCampaign[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [toast, setToast] = useState<Toast | null>(null);
  // 進行中の操作キー集合。行ごとに独立して二重送信を防ぐ（1 文字列だと別行操作で上書きされる）。
  const [busyActions, setBusyActions] = useState<Set<string>>(new Set());
  // 顧客個票（顧客360）ドロワーで開いている契約 ID。null なら閉じている。
  const [detailContractId, setDetailContractId] = useState<string | null>(null);

  /** 契約一覧と請求/回収状況を再取得して画面に反映する。 */
  const refresh = useCallback(async () => {
    const [c, s, d] = await Promise.all([
      api.listContracts(),
      api.listCollectionStates(),
      // 督促だけの障害で全画面を止めないよう段階的に劣化させる（契約/回収は表示を維持）。
      api.listDunningCampaigns().catch(() => []),
    ]);
    setContracts(c);
    setCollections(s);
    setDunning(d);
  }, []);

  useEffect(() => {
    let active = true;
    refresh()
      .catch((err: unknown) => {
        if (!active) return;
        setError(err instanceof Error ? err.message : "データの取得に失敗しました");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [refresh]);

  /** トーストを表示する（一定時間後に自動で消える）。 */
  const notify = useCallback((message: string, kind: "success" | "error") => {
    setToast({ message, kind });
  }, []);

  useEffect(() => {
    if (!toast) return;
    const id = setTimeout(() => setToast(null), 3600);
    return () => clearTimeout(id);
  }, [toast]);

  const metrics = useMemo(() => {
    const activeContracts = contracts.filter((c) => c.status === "active");
    const trialing = contracts.filter((c) => c.status === "trialing").length;
    const mrr = activeContracts.reduce((sum, c) => sum + c.monthlyFee.amount, 0);
    const inCollection = collections.filter((s) => s.status === "in_collection");
    const inCollectionAmount = inCollection.reduce((sum, s) => sum + s.amount.amount, 0);
    const paid = collections.filter((s) => s.status === "paid").length;
    const billed = collections.reduce((sum, s) => sum + s.amount.amount, 0);
    return {
      activeCount: activeContracts.length,
      trialing,
      mrr,
      inCollectionCount: inCollection.length,
      inCollectionAmount,
      paid,
      billed,
    };
  }, [contracts, collections]);

  /** 指定契約の請求を実行し、完了後に一覧を更新する。実行中は当該行のボタンを抑止する。 */
  const triggerBilling = useCallback(
    async (contractId: string) => {
      const key = `bill-${contractId}`;
      setBusyActions((prev) => new Set(prev).add(key));
      try {
        await api.triggerBilling(contractId);
        await refresh();
        notify(`${contractId} の請求を実行しました`, "success");
      } catch (err) {
        notify(err instanceof Error ? err.message : "請求実行に失敗しました", "error");
      } finally {
        setBusyActions((prev) => {
          const next = new Set(prev);
          next.delete(key);
          return next;
        });
      }
    },
    [refresh, notify],
  );

  const meta = pageMeta[view];
  const showContracts = view === "dashboard" || view === "contracts";
  const showCollections = view === "dashboard" || view === "collections";

  return (
    <div className="layout">
      <Sidebar view={view} onChange={setView} />

      <div className="layout__main">
        <header className="topbar">
          <div>
            <h1 className="topbar__title">{meta.title}</h1>
            <p className="topbar__subtitle">{meta.subtitle}</p>
          </div>
          <div className="topbar__right">
            <span className="chip">subscope</span>
            <span className="avatar">D</span>
          </div>
        </header>

        <main className="content">
          {loading ? (
            <div className="state">読み込み中…</div>
          ) : error ? (
            <div className="state state--error">エラー: {error}</div>
          ) : (
            <>
              {view === "dashboard" && (
                <section className="metrics">
                  <MetricCard
                    label="有効契約数"
                    value={`${metrics.activeCount} 件`}
                    sub={`トライアル ${metrics.trialing} 件`}
                    icon={<IconUsers />}
                  />
                  <MetricCard
                    label="月間経常収益 (MRR)"
                    value={formatMoney({ amount: metrics.mrr, currency: "JPY" })}
                    sub="有効契約ベース"
                    icon={<IconRevenue />}
                  />
                  <MetricCard
                    label="回収中"
                    value={`${metrics.inCollectionCount} 件`}
                    sub={formatMoney({ amount: metrics.inCollectionAmount, currency: "JPY" })}
                    icon={<IconAlert />}
                  />
                  <MetricCard
                    label="入金済み"
                    value={`${metrics.paid} 件`}
                    sub={`請求総額 ${formatMoney({ amount: metrics.billed, currency: "JPY" })}`}
                    icon={<IconCheck />}
                  />
                </section>
              )}

              {view === "operations" && (
                <Operations api={api} notify={notify} refresh={refresh} />
              )}

              {view === "settlement" && (
                <Settlements api={api} notify={notify} refresh={refresh} />
              )}

              {showContracts && (
                <section className="card">
                  <div className="card__head">
                    <h2 className="card__title">契約一覧</h2>
                    <span className="card__count">{contracts.length} 件</span>
                  </div>
                  <div className="table-wrap">
                    <table className="table">
                      <thead>
                        <tr>
                          <th>契約ID</th>
                          <th>会員</th>
                          <th>請求先</th>
                          <th className="ar">月会費</th>
                          <th>状態</th>
                          <th className="ar">操作</th>
                        </tr>
                      </thead>
                      <tbody>
                        {contracts.length === 0 ? (
                          <tr className="empty-row">
                            <td colSpan={6}>契約がありません。「登録・操作」から登録してください。</td>
                          </tr>
                        ) : (
                          contracts.map((c) => (
                            <tr key={c.id}>
                              <td className="mono">
                                <button
                                  type="button"
                                  className="link-btn"
                                  onClick={() => setDetailContractId(c.id)}
                                  title="顧客個票を開く"
                                >
                                  {c.id}
                                </button>
                              </td>
                              <td className="strong">{c.memberName}</td>
                              <td className="mono muted">{c.billingAccountId}</td>
                              <td className="ar num">{formatMoney(c.monthlyFee)}</td>
                              <td>
                                <StatusPill
                                  label={contractStatusLabel(c.status)}
                                  tone={contractStatusTone(c.status)}
                                />
                              </td>
                              <td className="ar">
                                <button
                                  type="button"
                                  className="btn btn--sm btn--ghost"
                                  disabled={busyActions.has(`bill-${c.id}`)}
                                  onClick={() => triggerBilling(c.id)}
                                >
                                  {busyActions.has(`bill-${c.id}`) ? "実行中…" : "請求実行"}
                                </button>
                              </td>
                            </tr>
                          ))
                        )}
                      </tbody>
                    </table>
                  </div>
                </section>
              )}

              {view === "dunning" && (
                <section className="card">
                  <div className="card__head">
                    <h2 className="card__title">督促キャンペーン</h2>
                    <span className="card__count">{dunning.length} 件</span>
                  </div>
                  <div className="table-wrap">
                    <table className="table">
                      <thead>
                        <tr>
                          <th>キャンペーンID</th>
                          <th>請求ID</th>
                          <th>請求先</th>
                          <th>状態</th>
                          <th className="ar">進捗</th>
                          <th>次のチャネル</th>
                        </tr>
                      </thead>
                      <tbody>
                        {dunning.length === 0 ? (
                          <tr className="empty-row">
                            <td colSpan={6}>
                              督促はありません。決済失敗・エスカレーションで起票されます。
                            </td>
                          </tr>
                        ) : (
                          dunning.map((d) => (
                            <tr key={d.campaignId}>
                              <td className="mono">{d.campaignId}</td>
                              <td className="mono muted">{d.invoiceId}</td>
                              <td className="mono muted">{d.account}</td>
                              <td>
                                <StatusPill
                                  label={dunningStatusLabel(d.status)}
                                  tone={dunningStatusTone(d.status)}
                                />
                              </td>
                              <td className="ar num">
                                {d.stepsTriggered} / {d.stepsTotal}
                              </td>
                              <td>{dunningChannelLabel(d.nextChannel)}</td>
                            </tr>
                          ))
                        )}
                      </tbody>
                    </table>
                  </div>
                </section>
              )}

              {showCollections && (
                <section className="card">
                  <div className="card__head">
                    <h2 className="card__title">請求・回収状況</h2>
                    <span className="card__count">{collections.length} 件</span>
                  </div>
                  <div className="table-wrap">
                    <table className="table">
                      <thead>
                        <tr>
                          <th>請求ID</th>
                          <th>契約ID</th>
                          <th className="ar">金額</th>
                          <th>状況</th>
                        </tr>
                      </thead>
                      <tbody>
                        {collections.length === 0 ? (
                          <tr className="empty-row">
                            <td colSpan={4}>請求がありません。契約一覧から「請求実行」してください。</td>
                          </tr>
                        ) : (
                          collections.map((s) => (
                            <tr key={s.invoiceId}>
                              <td className="mono">{s.invoiceId}</td>
                              <td className="mono muted">{s.contractId}</td>
                              <td className="ar num">{formatMoney(s.amount)}</td>
                              <td>
                                <StatusPill
                                  label={collectionStatusLabel(s.status)}
                                  tone={collectionStatusTone(s.status)}
                                />
                              </td>
                            </tr>
                          ))
                        )}
                      </tbody>
                    </table>
                  </div>
                </section>
              )}
            </>
          )}
        </main>
      </div>

      <CustomerDrawer
        api={api}
        contractId={detailContractId}
        onClose={() => setDetailContractId(null)}
      />

      {toast && <div className={`toast toast--${toast.kind}`}>{toast.message}</div>}
    </div>
  );
}

export default App;
