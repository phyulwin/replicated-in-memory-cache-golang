# syntax=docker/dockerfile:1

############################
# 1) Build binaries
############################
FROM golang:1.22-alpine AS builder
WORKDIR /src

# Cache deps early
COPY go.mod ./
# If you have a go.sum, copy it too:
# COPY go.sum ./
RUN go mod download

# App source
COPY . .

# Build static-ish binaries for Linux
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o /out/cache-node ./cmd/cache-node
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o /out/cachectl   ./cmd/cachectl

############################
# 2) Runtime: server image
############################
FROM alpine:3.20 AS node
RUN adduser -D -u 10001 app
COPY --from=builder /out/cache-node /usr/local/bin/cache-node
EXPOSE 8081
USER app

# Simple HTTP healthcheck (needs busybox wget in Alpine)
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -q -O - http://127.0.0.1:8081/health || exit 1

ENTRYPOINT ["/usr/local/bin/cache-node"]

############################
# 3) Runtime: CLI image
############################
FROM alpine:3.20 AS cli
COPY --from=builder /out/cachectl /usr/local/bin/cachectl
ENTRYPOINT ["/usr/local/bin/cachectl"]
