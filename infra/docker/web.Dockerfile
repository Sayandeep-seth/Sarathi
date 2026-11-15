# syntax=docker/dockerfile:1
FROM node:22-alpine AS build
WORKDIR /w
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# Runs as uid 101, listens on 8080, needs no capabilities.
FROM nginxinc/nginx-unprivileged:1.27-alpine
ENV API_UPSTREAM=api:8080 NGINX_ENVSUBST_FILTER=API_UPSTREAM
COPY infra/docker/web.security-headers.conf /etc/nginx/snippets/security-headers.conf
COPY infra/docker/web.nginx.conf.template /etc/nginx/templates/default.conf.template
COPY --from=build /w/dist /usr/share/nginx/html
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --retries=3 CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
