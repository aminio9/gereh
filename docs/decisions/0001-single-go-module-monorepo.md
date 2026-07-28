# ADR 0001: Single Go module in a microservices monorepo

## Decision

Use one root Go module while keeping every deployable service in a dedicated
directory with its own `internal/` packages, migrations and Dockerfile.

## Why

- One dependency graph and security-upgrade process
- Fast refactoring before service contracts stabilize
- Go `internal/` rules prevent cross-service domain imports
- Services remain separately buildable and deployable

A microservice boundary is a runtime, data and ownership boundary; it does not
require a separate Git repository or Go module.
