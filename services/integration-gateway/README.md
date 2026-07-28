# integration-gateway

Credential-safe access to GitHub, Slack, Google, MCP and customer APIs.

Default local HTTP port: `8090`.

This directory is an independently deployable service boundary. Its `internal/`
packages must not be imported by another service.
