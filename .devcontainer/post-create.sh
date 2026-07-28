#!/usr/bin/env bash
set -Eeuo pipefail

# Named Docker volumes are initially created as root.
# Make Go and pnpm caches writable by the vscode user.
sudo mkdir -p   /home/vscode/.cache/go-build   /go/pkg/mod   /home/vscode/.local/share/pnpm/store   /home/vscode/go/bin

sudo chown -R "$(id -u):$(id -g)"   /home/vscode/.cache   /home/vscode/.local   /home/vscode/go   /go/pkg/mod

export GOCACHE=/home/vscode/.cache/go-build
export GOMODCACHE=/go/pkg/mod
export GOPATH=/home/vscode/go
export PNPM_HOME=/home/vscode/.local/share/pnpm
export PATH="${PNPM_HOME}:${GOPATH}/bin:/usr/local/go/bin:/usr/local/bin:${PATH}"

for shell_file in "${HOME}/.bashrc" "${HOME}/.profile"; do
  touch "${shell_file}"
  if ! grep -Fq 'export PATH="$HOME/.local/share/pnpm:$HOME/go/bin:$PATH"' "${shell_file}"; then
    echo 'export PATH="$HOME/.local/share/pnpm:$HOME/go/bin:$PATH"' >> "${shell_file}" 
  fi
done


# Docker named volumes are initially mounted with root ownership.
# Repair cache ownership before running Go or pnpm as the vscode user.
sudo mkdir -p \
  /home/vscode/.cache/go-build \
  /go/pkg/mod \
  /home/vscode/.local/share/pnpm/store \
  /home/vscode/go/bin

sudo chown -R "$(id -u):$(id -g)" \
  /home/vscode/.cache \
  /home/vscode/.local \
  /home/vscode/go \
  /go/pkg/mod

export GOCACHE=/home/vscode/.cache/go-build
export GOMODCACHE=/go/pkg/mod
export GOPATH=/home/vscode/go
export PNPM_HOME=/home/vscode/.local/share/pnpm
export PATH="${PNPM_HOME}:${GOPATH}/bin:${PATH}"

export PATH="${HOME}/go/bin:/usr/local/go/bin:/usr/local/bin:${PATH}"
export GO111MODULE=on
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export NPM_CONFIG_REGISTRY="${NPM_CONFIG_REGISTRY:-https://registry.npmmirror.com}"
export PIP_INDEX_URL="${PIP_INDEX_URL:-https://mirrors.ustc.edu.cn/pypi/simple}"
go env -w GO111MODULE=on GOPROXY="${GOPROXY}"
npm config set registry "${NPM_CONFIG_REGISTRY}"
pnpm config set registry "${NPM_CONFIG_REGISTRY}"
pnpm config set store-dir "${HOME}/.local/share/pnpm/store"
mkdir -p "${HOME}/.config/pip"
printf '%s\n' '[global]' "index-url = ${PIP_INDEX_URL}" 'trusted-host = mirrors.ustc.edu.cn' 'timeout = 60' > "${HOME}/.config/pip/pip.conf"
git config --global --add safe.directory /workspaces/gereh-platform
install_go_tool() {
  local command_name="$1" package="$2"
  command -v "${command_name}" >/dev/null 2>&1 && { echo "✓ ${command_name}"; return; }
  echo "Installing ${command_name} through ${GOPROXY}..."
  go install "${package}"
}
install_go_tool air github.com/air-verse/air@v1.65.1
install_go_tool buf github.com/bufbuild/buf/cmd/buf@v1.72.0
install_go_tool golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4
install_go_tool grpcurl github.com/fullstorydev/grpcurl/cmd/grpcurl@v1.9.3
install_go_tool protoc-gen-go google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10
install_go_tool protoc-gen-go-grpc google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
install_go_tool sqlc github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
install_go_tool task github.com/go-task/task/v3/cmd/task@v3.51.1
[[ "${GEREH_INSTALL_INFRA_CLIS:-true}" == true ]] && install-infra-cli-tools
go mod download
pnpm install
buf lint
echo "Development container ready. Run: task doctor"
