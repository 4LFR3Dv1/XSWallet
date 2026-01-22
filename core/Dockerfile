# XS Wallet Go Core - Dockerfile
# Multi-stage build for minimal production image

# ============================================================================
# Builder Stage
# ============================================================================
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git make gcc musl-dev

WORKDIR /app

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with optimizations
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" \
    -o xscore \
    ./cmd/xscore

# ============================================================================
# Runtime Stage
# ============================================================================
FROM alpine:3.19

RUN apk add --no-cache ca-certificates sqlite

# Create non-root user
RUN adduser -D -u 1000 xswallet

WORKDIR /app

# Copy binary
COPY --from=builder /app/xscore /app/xscore

# Create data directory
RUN mkdir -p /data && chown xswallet:xswallet /data

USER xswallet

VOLUME ["/data"]

EXPOSE 9735

ENTRYPOINT ["/app/xscore"]
CMD ["--datadir=/data", "--port=9735"]
