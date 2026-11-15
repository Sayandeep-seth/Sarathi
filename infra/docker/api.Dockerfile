# syntax=docker/dockerfile:1
FROM golang:1.27 AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY backend/ ./
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/api ./cmd/api

# Distroless: no shell, no package manager, runs as uid 65532.
FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=dev
ENV SARATHI_VERSION=${VERSION}
COPY --from=build /out/api /api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]
