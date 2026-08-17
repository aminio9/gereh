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
  sigs.k8s.io/controller-tools/cmd/controller-gen

controller_gen="${tool_bin}/controller-gen"
if [ ! -x "${controller_gen}" ] && [ -x "${controller_gen}.exe" ]; then
  controller_gen="${controller_gen}.exe"
fi

test -x "${controller_gen}"

mkdir -p \
  "${repo_root}/deploy/crds"

rm -f \
  "${repo_root}/deploy/crds/runtime.gereh.ai_runtimecells.yaml"

cd "${repo_root}"

"${controller_gen}" \
  object \
  paths="./platform/go/runtimecell/api/v1"

"${controller_gen}" \
  crd:crdVersions=v1 \
  paths="./platform/go/runtimecell/api/v1" \
  output:crd:artifacts:config=deploy/crds

gofmt -w \
  platform/go/runtimecell/api/v1
