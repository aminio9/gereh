#!/usr/bin/env bash

set -Eeuo pipefail

repo_root="$(
  cd "$(dirname "${BASH_SOURCE[0]}")/../.."
  pwd
)"

tool_bin="$(mktemp -d)"

cleanup() {
  rm -rf "${tool_bin}"
}

trap cleanup EXIT

GOBIN="${tool_bin}" \
  go -C "${repo_root}/tools" install \
  github.com/bufbuild/buf/cmd/buf

GOBIN="${tool_bin}" \
  go -C "${repo_root}/tools" install \
  google.golang.org/protobuf/cmd/protoc-gen-go

GOBIN="${tool_bin}" \
  go -C "${repo_root}/tools" install \
  google.golang.org/grpc/cmd/protoc-gen-go-grpc

export PATH="${tool_bin}:${PATH}"

cd "${repo_root}"

buf lint

buf format \
  --diff \
  --exit-code

rm -rf gen/go

buf generate

gofmt -w gen/go
