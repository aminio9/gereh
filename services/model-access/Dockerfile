FROM golang:1.26-bookworm AS build
WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/model-access \
    ./services/model-access/cmd/model-access

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/model-access /model-access
USER nonroot:nonroot
ENTRYPOINT ["/model-access"]
