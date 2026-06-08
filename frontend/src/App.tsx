import { useEffect, useState } from "react";
import { api } from "./api/client";
import type { Contract, CollectionState } from "./api/types";
import {
  collectionStatusLabel,
  contractStatusLabel,
  formatMoney,
} from "./format";
import "./App.css";

function App() {
  const [contracts, setContracts] = useState<Contract[]>([]);
  const [collections, setCollections] = useState<CollectionState[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;
    Promise.all([api.listContracts(), api.listCollectionStates()]).then(
      ([c, s]) => {
        if (!active) return;
        setContracts(c);
        setCollections(s);
        setLoading(false);
      },
    );
    return () => {
      active = false;
    };
  }, []);

  return (
    <main className="app">
      <header className="app__header">
        <h1>subscope</h1>
        <p className="app__subtitle">サブスクリプション業務ダッシュボード（モックデータ）</p>
      </header>

      {loading ? (
        <p>読み込み中…</p>
      ) : (
        <>
          <section className="card">
            <h2>契約一覧</h2>
            <table>
              <thead>
                <tr>
                  <th>契約ID</th>
                  <th>会員</th>
                  <th>請求先</th>
                  <th>月会費</th>
                  <th>状態</th>
                </tr>
              </thead>
              <tbody>
                {contracts.map((c) => (
                  <tr key={c.id}>
                    <td>{c.id}</td>
                    <td>{c.memberName}</td>
                    <td>{c.billingAccountId}</td>
                    <td className="num">{formatMoney(c.monthlyFee)}</td>
                    <td>
                      <span className={`badge badge--${c.status}`}>
                        {contractStatusLabel(c.status)}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>

          <section className="card">
            <h2>請求・回収状況</h2>
            <table>
              <thead>
                <tr>
                  <th>請求ID</th>
                  <th>契約ID</th>
                  <th>金額</th>
                  <th>状況</th>
                </tr>
              </thead>
              <tbody>
                {collections.map((s) => (
                  <tr key={s.invoiceId}>
                    <td>{s.invoiceId}</td>
                    <td>{s.contractId}</td>
                    <td className="num">{formatMoney(s.amount)}</td>
                    <td>
                      <span className={`badge badge--${s.status}`}>
                        {collectionStatusLabel(s.status)}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>
        </>
      )}
    </main>
  );
}

export default App;
