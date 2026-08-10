#!/usr/bin/env bash

set -Eeuo pipefail

# ---------------------------------------------------------------------------
# Environment
# These values should match remoteEnv in devcontainer.json.
# ---------------------------------------------------------------------------

export GOPATH="${GOPATH:-${HOME}/go}"
export GOCACHE="${GOCACHE:-${HOME}/.cache/go-build}"
export GOMODCACHE="${GOMODCACHE:-/go/pkg/mod}"
export PNPM_HOME="${PNPM_HOME:-${HOME}/.local/share/pnpm}"

export PATH="${PNPM_HOME}:${GOPATH}/bin:/usr/local/go/bin:/usr/local/bin:${PATH}"

export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export NPM_CONFIG_REGISTRY="${NPM_CONFIG_REGISTRY:-https://registry.npmmirror.com}"
export PIP_INDEX_URL="${PIP_INDEX_URL:-https://mirrors.ustc.edu.cn/pypi/simple}"

readonly USER_ID="$(id -u)"
readonly GROUP_ID="$(id -g)"

# ---------------------------------------------------------------------------
# Named-volume permissions
# Docker named volumes can initially be owned by root.
# ---------------------------------------------------------------------------

sudo mkdir -p \
  "${GOCACHE}" \
  "${GOMODCACHE}" \
  "${PNPM_HOME}/store" \
  "${GOPATH}/bin" \
  "${HOME}/.config/pip"

sudo chown -R "${USER_ID}:${GROUP_ID}" \
  "${HOME}/.cache" \
  "${HOME}/.local" \
  "${GOPATH}" \
  "${GOMODCACHE}"

# ---------------------------------------------------------------------------
# Package mirrors
# ---------------------------------------------------------------------------

go env -w \
  GOPATH="${GOPATH}" \
  GOCACHE="${GOCACHE}" \
  GOMODCACHE="${GOMODCACHE}" \
  GOPROXY="${GOPROXY}"

npm config set registry "${NPM_CONFIG_REGISTRY}"
pnpm config set registry "${NPM_CONFIG_REGISTRY}"
pnpm config set store-dir "${PNPM_HOME}/store"

cat >"${HOME}/.config/pip/pip.conf" <<EOF
[global]
index-url = ${PIP_INDEX_URL}
trusted-host = mirrors.ustc.edu.cn
timeout = 60
EOF

git config --global --add safe.directory /workspaces/gereh-platform

# ---------------------------------------------------------------------------
# Go tool installer
#
# Always run `go install`.
# Go's build and module caches make repeated installation relatively cheap,
# and changing a pinned version will correctly update the binary.
# ---------------------------------------------------------------------------

install_go_tool() {
  local command_name="$1"
  local package="$2"

  echo
  echo "Installing ${command_name}: ${package}"

  GOBIN="${GOPATH}/bin" go install "${package}"

  if ! command -v "${command_name}" >/dev/null 2>&1; then
    echo "ERROR: ${command_name} was installed but is not available on PATH." >&2
    return 1
  fi

  echo "✓ ${command_name}: $(command -v "${command_name}")"
}

# ---------------------------------------------------------------------------
# Core editor tools
#
# The VS Code Go extension can manage these automatically. Installing them
# here also makes them available immediately in terminals and other editors.
# Pin exact versions if you require fully reproducible container rebuilds.
# ---------------------------------------------------------------------------

install_go_tool gopls \
  golang.org/x/tools/gopls@latest

install_go_tool dlv \
  github.com/go-delve/delve/cmd/dlv@latest

# Optional Go development helpers.
install_go_tool goimports \
  golang.org/x/tools/cmd/goimports@latest

install_go_tool gotests \
  github.com/cweill/gotests/gotests@latest

install_go_tool impl \
  github.com/josharian/impl@latest

install_go_tool gomodifytags \
  github.com/fatih/gomodifytags@latest

install_go_tool staticcheck \
  honnef.co/go/tools/cmd/staticcheck@latest

install_go_tool govulncheck \
  golang.org/x/vuln/cmd/govulncheck@latest

# ---------------------------------------------------------------------------
# Project tools
# ---------------------------------------------------------------------------

install_go_tool air \
  github.com/air-verse/air@v1.65.1

install_go_tool buf \
  github.com/bufbuild/buf/cmd/buf@v1.72.0

install_go_tool golangci-lint \
  github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4

install_go_tool grpcurl \
  github.com/fullstorydev/grpcurl/cmd/grpcurl@v1.9.3

# Atlas CLI. Note: the ariga.io/atlas/cmd/atlas module is versioned separately
# from the main ariga.io/atlas module (v0.13.x tags), and it pins an ancient
# golang.org/x/tools that no longer compiles with Go 1.26+. Build it with an
# older toolchain via GOTOOLCHAIN (auto-downloaded once, cached thereafter).
# GOSUMDB is forced on because GOTOOLCHAIN downloads require sumdb verification.
GOTOOLCHAIN=go1.24.5 GOSUMDB=sum.golang.org install_go_tool atlas \
  ariga.io/atlas/cmd/atlas@v0.13.1

install_go_tool protoc-gen-go \
  google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10

install_go_tool protoc-gen-go-grpc \
  google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

install_go_tool sqlc \
  github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1

install_go_tool task \
  github.com/go-task/task/v3/cmd/task@v3.51.1

# Optional infrastructure tools supplied by your image or another script.
if [[ "${GEREH_INSTALL_INFRA_CLIS:-true}" == "true" ]]; then
  if command -v install-infra-cli-tools >/dev/null 2>&1; then
    install-infra-cli-tools
  else
    echo "Skipping infrastructure CLIs: install-infra-cli-tools was not found."
  fi
fi

# ---------------------------------------------------------------------------
# Project dependencies
# ---------------------------------------------------------------------------

if [[ -f go.work || -f go.mod ]]; then
  echo
  echo "Downloading Go dependencies..."
  go mod download
else
  echo "Skipping go mod download: no go.work or go.mod in $(pwd)."
fi

if [[ -f pnpm-lock.yaml ]]; then
  echo
  echo "Installing pnpm dependencies..."
  pnpm install --frozen-lockfile
elif [[ -f package.json ]]; then
  echo
  echo "Installing pnpm dependencies without a lockfile..."
  pnpm install
else
  echo "Skipping pnpm install: no package.json found."
fi

# ---------------------------------------------------------------------------
# Verification
# ---------------------------------------------------------------------------

echo
echo "Installed Go environment:"
go version
go env GOPATH GOCACHE GOMODCACHE GOPROXY

echo
echo "Editor tools:"
gopls version
dlv version

echo
echo "Development container is ready."
echo "Run project checks separately with: task doctor"
