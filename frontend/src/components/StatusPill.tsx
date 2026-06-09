import type { Tone } from "../format";

interface Props {
  label: string;
  tone: Tone;
}

/** Stripe 風のステータスピル（色付きドット + ラベル）。 */
export function StatusPill({ label, tone }: Props) {
  return (
    <span className={`pill pill--${tone}`}>
      <span className="pill__dot" />
      {label}
    </span>
  );
}
