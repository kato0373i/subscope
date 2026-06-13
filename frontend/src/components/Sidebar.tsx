import type { ReactNode } from "react";
import type { View } from "../views";
import {
  IconCollection,
  IconContract,
  IconDashboard,
  IconDunning,
  IconOperations,
} from "./icons";

interface Props {
  view: View;
  onChange: (view: View) => void;
}

const items: { key: View; label: string; icon: ReactNode }[] = [
  { key: "dashboard", label: "ダッシュボード", icon: <IconDashboard /> },
  { key: "operations", label: "登録・操作", icon: <IconOperations /> },
  { key: "contracts", label: "契約", icon: <IconContract /> },
  { key: "collections", label: "請求・回収", icon: <IconCollection /> },
  { key: "dunning", label: "督促", icon: <IconDunning /> },
];

/** 左サイドバー。ブランド + ナビゲーション。 */
export function Sidebar({ view, onChange }: Props) {
  return (
    <aside className="sidebar">
      <div className="brand">
        <span className="brand__mark">S</span>
        <span className="brand__name">subscope</span>
      </div>

      <nav className="nav">
        {items.map((item) => (
          <button
            key={item.key}
            type="button"
            className={`nav__item${view === item.key ? " nav__item--active" : ""}`}
            onClick={() => onChange(item.key)}
          >
            <span className="nav__icon">{item.icon}</span>
            {item.label}
          </button>
        ))}
      </nav>

      <div className="sidebar__foot">
        <span className="sidebar__env">subscope</span>
        <span className="sidebar__hint">会費 Pay 型 SaaS</span>
      </div>
    </aside>
  );
}
