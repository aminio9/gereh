# syntax=docker/dockerfile:1.12

ARG GO_IMAGE=golang:1.26.5-bookworm
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian12:nonroot

FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS dependencies

WORKDIR /src

ENV GOTOOLCHAIN=local
ENV GOWORK=off

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download


FROM dependencies AS build

ARG SERVICE
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG REVISION=unknown
ARG BUILD_DATE=unknown

COPY . .

RUN test -n "${SERVICE}" \
    && test -d "./services/${SERVICE}/cmd/${SERVICE}"

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS="${TARGETOS:-linux}" \
    GOARCH="${TARGETARCH:-amd64}" \
    go build \
      -buildvcs=false \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/service \
      "./services/${SERVICE}/cmd/${SERVICE}"


FROM ${RUNTIME_IMAGE} AS runtime

ARG SERVICE
ARG VERSION=dev
ARG REVISION=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="Gereh ${SERVICE}"
LABEL org.opencontainers.image.description="Gereh service container"
LABEL org.opencontainers.image.source="https://github.com/aminio9/gereh"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.revision="${REVISION}"
LABEL org.opencontainers.image.created="${BUILD_DATE}"
LABEL org.opencontainers.image.vendor="Gereh"
LABEL org.opencontainers.image.licenses="Proprietary"

ENV HTTP_ADDRESS=:8080
ENV SERVICE_VERSION=${VERSION}

WORKDIR /app

COPY --from=build --chown=nonroot:nonroot \
  /out/service \
  /app/service

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/app/service"]
