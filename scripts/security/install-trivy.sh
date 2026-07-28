#!/usr/bin/env bash
set -Eeuo pipefail

version="${TRIVY_VERSION:-0.70.0}"
destination="${1:-${HOME}/.local/bin}"

case "$(uname -m)" in
  x86_64)
    architecture="64bit"
    ;;
  aarch64 | arm64)
    architecture="ARM64"
    ;;
  *)
    echo "Unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

archive="trivy_${version}_Linux-${architecture}.tar.gz"
checksums="trivy_${version}_checksums.txt"

base_url="https://github.com/aquasecurity/trivy/releases/download/v${version}"

workdir="$(mktemp -d)"

cleanup() {
  rm -rf "${workdir}"
}

trap cleanup EXIT

curl \
  --fail \
  --show-error \
  --location \
  --retry 5 \
  --retry-all-errors \
  --output "${workdir}/${archive}" \
  "${base_url}/${archive}"

curl \
  --fail \
  --show-error \
  --location \
  --retry 5 \
  --retry-all-errors \
  --output "${workdir}/${checksums}" \
  "${base_url}/${checksums}"

expected_checksum="$(
  awk \
    -v archive="${archive}" \
    '
      {
        filename = $2
        sub(/^\*/, "", filename)

        if (filename == archive) {
          print $1
          exit
        }
      }
    ' \
    "${workdir}/${checksums}"
)"

if [[ ! "${expected_checksum}" =~ ^[0-9a-fA-F]{64}$ ]]; then
  echo "Could not resolve the Trivy checksum." >&2
  exit 1
fi

printf '%s  %s\n' \
  "${expected_checksum}" \
  "${workdir}/${archive}" \
  | sha256sum --check -

tar \
  --extract \
  --gzip \
  --file "${workdir}/${archive}" \
  --directory "${workdir}" \
  trivy

install \
  -d \
  -m 0755 \
  "${destination}"

install \
  -m 0755 \
  "${workdir}/trivy" \
  "${destination}/trivy"

"${destination}/trivy" --version
