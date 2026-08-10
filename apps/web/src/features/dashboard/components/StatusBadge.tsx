import { statusLabel } from "../formatters";

interface StatusBadgeProps {
  status: string;
}

export function StatusBadge({ status }: StatusBadgeProps) {
  return (
    <span className="status-badge" data-status={status}>
      <span className="status-badge__dot" aria-hidden="true" />
      {statusLabel(status)}
    </span>
  );
}
