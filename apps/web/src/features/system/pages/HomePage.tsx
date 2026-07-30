import { useQuery } from "@tanstack/react-query";

import { getSystemStatus } from "../api/getSystemStatus";
import { SystemStatusCard } from "../components/SystemStatusCard";

export function HomePage() {
  const statusQuery = useQuery({
    queryKey: ["system", "status"],
    queryFn: getSystemStatus,
    retry: 1,
    staleTime: 30_000,
    refetchInterval: 60_000,
  });

  return (
    <main className="page-shell">
      <section className="hero-card">
        <p className="eyebrow">Gereh cloud platform</p>

        <h1>Coordinate autonomous AI teams with confidence.</h1>

        <p className="hero-card__description">
          The application workspace, service boundaries, and contract foundation are ready for
          tenant onboarding.
        </p>

        {statusQuery.isPending ? <div className="status-card">Checking the API…</div> : null}

        {statusQuery.isError ? (
          <div className="status-card status-card--error" role="alert">
            API unavailable: {statusQuery.error.message}
          </div>
        ) : null}

        {statusQuery.data ? <SystemStatusCard status={statusQuery.data} /> : null}
      </section>
    </main>
  );
}
