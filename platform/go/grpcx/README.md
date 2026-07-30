# Gereh gRPC foundation

This package provides shared gRPC client and server construction.

## Included behavior

- OpenTelemetry stats handlers for traces and metrics
- W3C trace and baggage propagation
- structured request completion logs
- panic recovery
- standard gRPC health service
- optional reflection for development
- bounded message sizes
- TLS by default
- explicit insecure mode for local development
- request, correlation, and tenant metadata propagation
- graceful shutdown

## Security rules

- Production clients must use TLS.
- Reflection must be disabled in production unless explicitly required.
- `x-tenant-id` is transport metadata, not proof of authorization.
- Authentication middleware must validate or overwrite tenant metadata.
- Do not configure global retries for non-idempotent RPC methods.
- Method-specific retry policies belong in generated client configuration.

## Server registration

```go
server, err := grpcx.NewServer(
    grpcx.DefaultServerConfig(),
    telemetry,
    logger,
)
if err != nil {
    return err
}

gerehv1.RegisterTenantServiceServer(
    server.GRPC(),
    implementation,
)
Client creation
config := grpcx.DefaultClientConfig(
    "dns:///tenant:9090",
)

connection, err := grpcx.NewClient(
    config,
    telemetry,
)
```
