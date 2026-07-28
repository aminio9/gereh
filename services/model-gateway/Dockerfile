FROM golang:1.26-bookworm AS build
WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/model-gateway \
    ./services/model-gateway/cmd/model-gateway

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/model-gateway /model-gateway
USER nonroot:nonroot
ENTRYPOINT ["/model-gateway"]
