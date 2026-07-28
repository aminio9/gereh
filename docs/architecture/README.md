# Architecture

Gereh separates four responsibility layers:

1. **Control plane** — tenants, companies, agents, work, policy and billing.
2. **Durable orchestration** — Temporal workflows and activities.
3. **Event backbone** — Kafka domain events and projections.
4. **Execution plane** — tenant-isolated OpenClaw and Hermes runtime cells.

Initial implementation order:

1. Tenant onboarding vertical slice
2. Company and agent definition
3. Model connection and model gateway
4. Runtime cell provisioning
5. Task execution and approval
6. Usage ledger and billing
