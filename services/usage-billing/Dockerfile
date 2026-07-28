FROM golang:1.26-bookworm AS build
WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/usage-billing \
    ./services/usage-billing/cmd/usage-billing

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/usage-billing /usage-billing
USER nonroot:nonroot
ENTRYPOINT ["/usage-billing"]
