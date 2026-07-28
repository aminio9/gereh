FROM golang:1.26-bookworm AS build
WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/execution-orchestrator \
    ./services/execution-orchestrator/cmd/execution-orchestrator

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/execution-orchestrator /execution-orchestrator
USER nonroot:nonroot
ENTRYPOINT ["/execution-orchestrator"]
