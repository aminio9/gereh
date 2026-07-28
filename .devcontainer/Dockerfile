ARG GO_TOOLCHAIN_IMAGE=golang:1.26-bookworm
ARG NODE_TOOLCHAIN_IMAGE=node:24-bookworm

FROM ${GO_TOOLCHAIN_IMAGE} AS go-toolchain
FROM ${NODE_TOOLCHAIN_IMAGE} AS node-toolchain

# MCR is outside Docker Hub, so use DaoCloud's explicit registry prefix.
FROM m.daocloud.io/mcr.microsoft.com/devcontainers/base:ubuntu-24.04

ARG NPM_REGISTRY=https://registry.npmmirror.com
ARG PYPI_INDEX=https://mirrors.ustc.edu.cn/pypi/simple
ARG GOPROXY_URL=https://goproxy.cn,direct

ENV DEBIAN_FRONTEND=noninteractive \
    GOPROXY=${GOPROXY_URL} \
    GO111MODULE=on \
    NPM_CONFIG_REGISTRY=${NPM_REGISTRY} \
    npm_config_registry=${NPM_REGISTRY} \
    PNPM_REGISTRY=${NPM_REGISTRY} \
    PIP_INDEX_URL=${PYPI_INDEX} \
    UV_DEFAULT_INDEX=${PYPI_INDEX} \
    PATH=/usr/local/go/bin:/usr/local/bin:${PATH}

# Go and Node come from Docker Hub images. Docker Desktop's ordered
# registry-mirrors provide DaoCloud -> 1ms -> XuanYuan fallback.
COPY --from=go-toolchain /usr/local/go /usr/local/go
COPY --from=node-toolchain /usr/local /usr/local

# Ubuntu 24.04 uses DEB822 sources. TUNA's ubuntu mirror is amd64/i386;
# other architectures use TUNA ubuntu-ports.
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    . /etc/os-release; \
    case "${arch}" in \
      amd64|i386) ubuntu_uri="https://mirrors.tuna.tsinghua.edu.cn/ubuntu" ;; \
      *) ubuntu_uri="https://mirrors.tuna.tsinghua.edu.cn/ubuntu-ports" ;; \
    esac; \
    cat > /etc/apt/sources.list.d/ubuntu.sources <<EOF
Types: deb
URIs: ${ubuntu_uri}
Suites: ${VERSION_CODENAME} ${VERSION_CODENAME}-updates ${VERSION_CODENAME}-backports ${VERSION_CODENAME}-security
Components: main restricted universe multiverse
Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg
EOF
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      bash-completion build-essential ca-certificates curl dnsutils git gnupg jq \
      make netcat-openbsd openssl postgresql-client protobuf-compiler python3 \
      python3-pip redis-tools shellcheck unzip xz-utils zip; \
    rm -rf /var/lib/apt/lists/*

# Docker CLI, Buildx and Compose are installed from Alibaba Cloud's Docker CE repo.
# Docker Desktop remains the daemon (Docker-outside-of-Docker).
RUN set -eux; \
    install -m 0755 -d /etc/apt/keyrings; \
    curl -fsSL https://mirrors.aliyun.com/docker-ce/linux/ubuntu/gpg \
      | gpg --dearmor -o /etc/apt/keyrings/docker.gpg; \
    chmod a+r /etc/apt/keyrings/docker.gpg; \
    arch="$(dpkg --print-architecture)"; \
    . /etc/os-release; \
    echo "deb [arch=${arch} signed-by=/etc/apt/keyrings/docker.gpg] https://mirrors.aliyun.com/docker-ce/linux/ubuntu ${VERSION_CODENAME} stable" \
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
trusted-host = mirrors.ustc.edu.cn
timeout = 60
EOF
RUN go env -w GO111MODULE=on GOPROXY="${GOPROXY_URL}"

COPY .devcontainer/scripts/github-download /usr/local/bin/github-download
COPY .devcontainer/scripts/github-clone /usr/local/bin/github-clone
COPY .devcontainer/scripts/install-infra-cli-tools /usr/local/bin/install-infra-cli-tools
RUN chmod +x /usr/local/bin/github-download /usr/local/bin/github-clone /usr/local/bin/install-infra-cli-tools

USER vscode
RUN mkdir -p /home/vscode/.config/pip /home/vscode/.local/share/pnpm/store \
    && printf '%s\n' '[global]' "index-url = ${PYPI_INDEX}" \
       'trusted-host = mirrors.ustc.edu.cn' 'timeout = 60' \
       > /home/vscode/.config/pip/pip.conf \
    && npm config set registry "${NPM_REGISTRY}" \
    && pnpm config set registry "${NPM_REGISTRY}" \
    && go env -w GO111MODULE=on GOPROXY="${GOPROXY_URL}"
USER root
