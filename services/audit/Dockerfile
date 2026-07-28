FROM golang:1.26-bookworm AS build
WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/audit \
    ./services/audit/cmd/audit

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/audit /audit
USER nonroot:nonroot
ENTRYPOINT ["/audit"]
