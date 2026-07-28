FROM golang:1.26-bookworm AS build
WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/policy-approval \
    ./services/policy-approval/cmd/policy-approval

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/policy-approval /policy-approval
USER nonroot:nonroot
ENTRYPOINT ["/policy-approval"]
