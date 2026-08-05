ARG GO_IMAGE=golang:1.26.5-bookworm
ARG RUNTIME_IMAGE=gcr.io/distroless/base-debian12:nonroot

FROM ${GO_IMAGE} AS builder

WORKDIR /src

COPY go.mod go.sum go.work go.work.sum ./
COPY tools/go.mod tools/go.sum ./tools/

RUN go mod download

COPY . .

ARG VERSION=dev
ARG REVISION=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 \
    GOOS=linux \
    go build \
      -trimpath \
      -ldflags="-s -w \
        -X main.version=${VERSION}" \
      -o /out/tenant \
      ./services/tenant/cmd/tenant

FROM ${RUNTIME_IMAGE}

WORKDIR /app

COPY --from=builder /out/tenant /app/tenant

USER nonroot:nonroot

ENTRYPOINT ["/app/tenant"]