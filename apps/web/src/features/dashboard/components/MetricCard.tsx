import type { LucideIcon } from "lucide-react";

import { formatNumber } from "../formatters";

interface MetricCardProps {
  icon: LucideIcon;
  label: string;
  value: number;
  detail: string;
  tone?: "neutral" | "good" | "warning" | "danger";
}

export function MetricCard({
  icon: Icon,
  label,
  value,
  detail,
  tone = "neutral",
}: MetricCardProps) {
  return (
    <article className="metric-card" data-tone={tone}>
      <div className="metric-card__icon" aria-hidden="true">
        <Icon size={18} />
      </div>

      <div className="metric-card__content">
        <p className="metric-card__label">{label}</p>
        <strong className="metric-card__value">{formatNumber(value)}</strong>
        <p className="metric-card__detail">{detail}</p>
      </div>
    </article>
  );
}
