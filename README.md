# Gereh Platform

Production-oriented bootstrap for rebuilding Gereh as a Go microservices platform with a React/TypeScript frontend.

## Core local stack

- Go 1.26
- Node.js 24 LTS
- pnpm 11
- React 19 + Vite 8
- PostgreSQL 18
- Redis 8
- Apache Kafka 4 in KRaft mode
- Temporal development server
- MinIO for local S3-compatible storage
- Optional Grafana OpenTelemetry LGTM stack

## Start in VS Code

1. Install Docker Desktop, VS Code, WSL 2, and the **Dev Containers** extension.
2. Store this repository inside the WSL filesystem, for example `~/src/gereh-platform`.
3. Open the folder in VS Code.
4. Run **Dev Containers: Reopen in Container**.
5. Wait for the post-create bootstrap.
6. In the container terminal:

```bash
task doctor
task dev:api
```

Open another terminal:

```bash
task dev:web
```

URLs:

- Web: http://localhost:5173
- API BFF: http://localhost:8080/v1/status
- Temporal UI: http://localhost:8233
- MinIO console: http://localhost:9001
- Grafana, when enabled: http://localhost:3001

## Infrastructure commands

```bash
task infra:up
task infra:ps
task infra:logs
task infra:observability
task infra:down
task infra:reset
```

`infra:reset` permanently removes local development volumes.

## Service development

```bash
task dev:service SERVICE=tenant
task dev:service SERVICE=organization-agent
task dev:service SERVICE=work-management
```

## Quality commands

```bash
task fmt
task generate
task lint
task test
task build
```

## Architectural rule

The monorepo is one Go module for dependency consistency, but each deployable service owns its own `internal/` domain and adapters. Services must not import another service's `internal/` packages or query another service's database.
