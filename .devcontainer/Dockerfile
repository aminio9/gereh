ARG GO_TOOLCHAIN_IMAGE=golang:1.26-bookworm
ARG NODE_TOOLCHAIN_IMAGE=node:24-bookworm

FROM ${GO_TOOLCHAIN_IMAGE} AS go-toolchain
FROM ${NODE_TOOLCHAIN_IMAGE} AS node-toolchain

FROM mcr.microsoft.com/devcontainers/base:ubuntu-24.04

ARG NPM_REGISTRY=https://registry.npmjs.org/
ARG PYPI_INDEX=https://pypi.org/simple
ARG GOPROXY_URL=https://proxy.golang.org,direct

ENV DEBIAN_FRONTEND=noninteractive \
    GOPROXY=${GOPROXY_URL} \
    GO111MODULE=on \
    NPM_CONFIG_REGISTRY=${NPM_REGISTRY} \
    npm_config_registry=${NPM_REGISTRY} \
    PNPM_REGISTRY=${NPM_REGISTRY} \
    PIP_INDEX_URL=${PYPI_INDEX} \
    UV_DEFAULT_INDEX=${PYPI_INDEX} \
    PATH=/usr/local/go/bin:/usr/local/bin:${PATH}

COPY --from=go-toolchain /usr/local/go /usr/local/go
COPY --from=node-toolchain /usr/local /usr/local

RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      bash-completion build-essential ca-certificates curl dnsutils git gnupg jq \
      make netcat-openbsd openssl postgresql-client protobuf-compiler python3 \
      python3-pip redis-tools shellcheck unzip xz-utils zip; \
    rm -rf /var/lib/apt/lists/*

# Docker CLI, Buildx and Compose installed from official Docker CE repo.
# Docker Desktop remains the daemon (Docker-outside-of-Docker).
RUN set -eux; \
    install -m 0755 -d /etc/apt/keyrings; \
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
      | gpg --dearmor -o /etc/apt/keyrings/docker.gpg; \
    chmod a+r /etc/apt/keyrings/docker.gpg; \
    arch="$(dpkg --print-architecture)"; \
    . /etc/os-release; \
    echo "deb [arch=${arch} signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu ${VERSION_CODENAME} stable" \
      > /etc/apt/sources.list.d/docker.list; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      docker-ce-cli docker-buildx-plugin docker-compose-plugin; \
    rm -rf /var/lib/apt/lists/*

RUN set -eux; \
    npm config set registry "${NPM_REGISTRY}"; \
    npm install --global pnpm@11.4.0 --registry="${NPM_REGISTRY}"; \
    pnpm config set registry "${NPM_REGISTRY}"; \
    cat > /etc/pip.conf <<EOF
[global]
index-url = ${PYPI_INDEX}
timeout = 60
EOF
RUN go env -w GO111MODULE=on GOPROXY="${GOPROXY_URL}"

COPY .devcontainer/scripts/install-infra-cli-tools /usr/local/bin/install-infra-cli-tools
RUN chmod +x /usr/local/bin/install-infra-cli-tools

USER vscode
RUN mkdir -p /home/vscode/.config/pip /home/vscode/.local/share/pnpm/store \
    && printf '%s\n' '[global]' "index-url = ${PYPI_INDEX}" \
       'timeout = 60' \
       > /home/vscode/.config/pip/pip.conf \
    && npm config set registry "${NPM_REGISTRY}" \
    && pnpm config set registry "${NPM_REGISTRY}" \
    && go env -w GO111MODULE=on GOPROXY="${GOPROXY_URL}"
USER vscode
