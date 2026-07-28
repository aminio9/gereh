# China mirrors and proxy

## Docker Desktop

Open `Docker Desktop → Settings → Docker Engine` and merge:

```json
{
  "registry-mirrors": [
    "https://docker.m.daocloud.io",
    "https://docker.1ms.run",
    "https://docker.xuanyuan.me"
  ],
  "features": { "buildkit": true }
}
```

Docker registry mirrors affect Docker Hub only. Prefix other registries explicitly:

```text
mcr.microsoft.com/x/y:tag       -> m.daocloud.io/mcr.microsoft.com/x/y:tag
ghcr.io/x/y:tag                 -> m.daocloud.io/ghcr.io/x/y:tag
quay.io/x/y:tag                 -> m.daocloud.io/quay.io/x/y:tag
registry.k8s.io/x/y:tag         -> m.daocloud.io/registry.k8s.io/x/y:tag
gcr.io/x/y:tag                  -> m.daocloud.io/gcr.io/x/y:tag
```

## Local GitHub proxy

Windows must allow LAN connections, not listen only on `127.0.0.1`.

Override before opening VS Code:

```bash
code .
```

For SOCKS5:

```bash
code .
```


```bash
github-clone https://github.com/owner/repository.git
github-download https://github.com/owner/repository/releases/download/v1/file.tar.gz /tmp/file.tar.gz
```

## Verify language mirrors

```bash
grep -R 'mirrors.tuna.tsinghua.edu.cn' /etc/apt/sources.list.d
grep -R 'mirrors.aliyun.com/docker-ce' /etc/apt/sources.list.d
npm config get registry
pnpm config get registry
python3 -m pip config list
go env GOPROXY
```
