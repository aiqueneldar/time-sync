# ─── Stage 1: Build ──────────────────────────────────────────────────────────
FROM golang:1.21-alpine AS builder

# Install build dependencies.
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy dependency files first for better layer caching.
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code.
COPY . .

# Build the binary with security hardening flags:
#   -trimpath       removes local file paths from the binary
#   -buildvcs=false omits VCS metadata
#   CGO_ENABLED=0   fully static binary, no libc dependency
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -buildvcs=false \
    -ldflags="-s -w -extldflags '-static'" \
    -o /timesync-server \
    ./cmd/server

# ─── Stage 2: Runtime ─────────────────────────────────────────────────────────
# Use a minimal scratch image – no shell, no package manager, no attack surface.
FROM scratch

# Copy only what is needed.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /timesync-server /timesync-server

# Run as a non-root user (UID 65532 = "nobody").
USER 65532:65532

# Expose the default port.
EXPOSE 8080

# Health check.
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD ["/timesync-server", "-health"] || exit 1

ENTRYPOINT ["/timesync-server"]
