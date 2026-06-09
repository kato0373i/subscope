import type { ReactNode } from "react";

interface Props {
  label: string;
  value: string;
  sub?: string;
  icon?: ReactNode;
}

/** ダッシュボード上部の KPI カード。 */
export function MetricCard({ label, value, sub, icon }: Props) {
  return (
    <div className="metric">
      <div className="metric__head">
        <span className="metric__label">{label}</span>
        {icon ? <span className="metric__icon">{icon}</span> : null}
      </div>
      <div className="metric__value">{value}</div>
      {sub ? <div className="metric__sub">{sub}</div> : null}
    </div>
  );
}
