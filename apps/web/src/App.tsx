import { useQuery } from "@tanstack/react-query";

type Status = {
  status: string;
  service: string;
  version: string;
};

async function loadStatus(): Promise<Status> {
  const response = await fetch("/v1/status");

  if (!response.ok) {
    throw new Error(`API returned ${response.status}`);
  }

  return response.json() as Promise<Status>;
}

export function App() {
  const status = useQuery({
    queryKey: ["api-status"],
    queryFn: loadStatus,
  });

  return (
    <main className="shell">
      <section className="card">
        <p className="eyebrow">Gereh cloud platform</p>
        <h1>Production architecture, clean foundation.</h1>
        <p>
          This frontend is connected to the Go API BFF and ready for the
          tenant onboarding vertical slice.
        </p>

        <div className="status">
          <span
            className={
              status.isSuccess ? "indicator indicator--ready" : "indicator"
            }
          />
          {status.isPending && "Checking API…"}
          {status.isError && `API unavailable: ${status.error.message}`}
          {status.isSuccess &&
            `${status.data.service} is ${status.data.status} (${status.data.version})`}
        </div>
      </section>
    </main>
  );
}
