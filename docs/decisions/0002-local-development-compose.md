# ADR 0002: Docker Compose and Dev Containers for local development

## Decision

Use one VS Code Dev Container as the coding environment and Docker Compose for
shared local infrastructure.

## Why

- Identical Linux toolchain on Windows, macOS and Linux
- No host installation of Go, Node, PostgreSQL, Kafka or Temporal
- Reproducible onboarding and CI parity
- Infrastructure remains separate from application processes
