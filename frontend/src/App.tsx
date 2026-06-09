import { useEffect, useMemo, useState } from "react";
import { api } from "./api/client";
import type { CollectionState, Contract } from "./api/types";
import {
  collectionStatusLabel,
  collectionStatusTone,
  contractStatusLabel,
  contractStatusTone,
  formatMoney,
} from "./format";
import type { View } from "./views";
import { Sidebar } from "./components/Sidebar";
import { MetricCard } from "./components/MetricCard";
import { StatusPill } from "./components/StatusPill";
import { IconAlert, IconCheck, IconRevenue, IconUsers } from "./components/icons";
import "./App.css";

const pageMeta: Record<View, { title: string; subtitle: string }> = {
  dashboard: { title: "ダッシュボード", subtitle: "サブスクリプション業務の概況" },
  contracts: { title: "契約", subtitle: "契約の一覧と状態" },
  collections: { title: "請求・回収", subtitle: "請求書ごとの入金・回収状況" },
};

function App() {
  const [view, setView] = useState<View>("dashboard");
  const [contracts, setContracts] = useState<Contract[]>([]);
  const [collections, setCollections] = useState<CollectionState[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    Promise.all([api.listContracts(), api.listCollectionStates()])
      .then(([c, s]) => {
        if (!active) return;
        setContracts(c);
        setCollections(s);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (!active) return;
        setError(err instanceof Error ? err.message : "データの取得に失敗しました");
        setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

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
            <span className="chip">モックデータ</span>
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
                        </tr>
                      </thead>
                      <tbody>
                        {contracts.map((c) => (
                          <tr key={c.id}>
                            <td className="mono">{c.id}</td>
                            <td className="strong">{c.memberName}</td>
                            <td className="mono muted">{c.billingAccountId}</td>
                            <td className="ar num">{formatMoney(c.monthlyFee)}</td>
                            <td>
                              <StatusPill
                                label={contractStatusLabel(c.status)}
                                tone={contractStatusTone(c.status)}
                              />
                            </td>
                          </tr>
                        ))}
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
                        {collections.map((s) => (
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
                        ))}
                      </tbody>
                    </table>
                  </div>
                </section>
              )}
            </>
          )}
        </main>
      </div>
    </div>
  );
}

export default App;
