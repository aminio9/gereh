# syntax=docker/dockerfile:1.12

ARG NODE_IMAGE=node:24.18.0-bookworm-slim
ARG NGINX_IMAGE=nginx:1.30.4-alpine

FROM ${NODE_IMAGE} AS dependencies

WORKDIR /src

ENV CI=true

RUN npm install --global pnpm@11.4.0

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY apps/web/package.json apps/web/package.json

RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install \
      --frozen-lockfile \
      --filter @gereh/web...


FROM dependencies AS build

COPY . .

RUN pnpm --filter @gereh/web build


FROM ${NGINX_IMAGE} AS runtime

ARG VERSION=dev
ARG REVISION=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="Gereh Web"
LABEL org.opencontainers.image.description="Gereh web application"
LABEL org.opencontainers.image.source="https://github.com/aminio9/gereh"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.revision="${REVISION}"
LABEL org.opencontainers.image.created="${BUILD_DATE}"
LABEL org.opencontainers.image.vendor="Gereh"
LABEL org.opencontainers.image.licenses="Proprietary"

RUN rm -f /etc/nginx/conf.d/default.conf \
    && chown -R nginx:nginx \
      /var/cache/nginx \
      /var/log/nginx \
      /etc/nginx/conf.d

COPY --chown=nginx:nginx \
  build/package/nginx.conf \
  /etc/nginx/nginx.conf

COPY --from=build --chown=nginx:nginx \
  /src/apps/web/dist \
  /usr/share/nginx/html

USER nginx

EXPOSE 8080

STOPSIGNAL SIGQUIT

CMD ["nginx", "-g", "daemon off;"]
