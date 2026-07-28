import type { SystemStatus } from "../api/getSystemStatus";

type SystemStatusCardProps = {
  readonly status: SystemStatus;
};

export function SystemStatusCard({ status }: SystemStatusCardProps) {
  return (
    <section className="status-card" aria-label="System status">
      <span className="status-card__indicator" aria-hidden="true" />

      <div>
        <strong>API BFF is healthy.</strong>
        <p>
          {status.service} · {status.version}
        </p>
      </div>
    </section>
  );
}
