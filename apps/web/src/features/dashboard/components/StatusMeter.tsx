import { formatNumber } from "../formatters";

interface StatusMeterProps {
  label: string;
  value: number;
  total: number;
  tone: "good" | "neutral" | "warning" | "danger";
}

export function StatusMeter({ label, value, total, tone }: StatusMeterProps) {
  const percentage = total <= 0 ? 0 : Math.min(100, Math.round((value / total) * 100));

  return (
    <div className="status-meter">
      <div className="status-meter__header">
        <span>{label}</span>
        <span>{formatNumber(value)}</span>
      </div>

      <div className="status-meter__track">
        <div className="status-meter__fill" data-tone={tone} style={{ width: `${percentage}%` }} />
      </div>
    </div>
  );
}
