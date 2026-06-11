# Multi-stage Dockerfile for LogLynx
# Builder stage: compiles a static binary for the target platform
FROM golang:1.25.8 AS builder

WORKDIR /src

# Copy go.mod and go.sum first to leverage Docker layer cache
COPY go.mod go.sum ./
RUN go mod download

# Copy rest of the sources
COPY . .

# Build the server binary (CGO enabled for sqlite/geoip native deps)
# TARGETPLATFORM and TARGETARCH are automatically set by Docker Buildx
ARG TARGETPLATFORM
ARG TARGETARCH
ARG LOGLYNX_USAGE_TELEMETRY_ENDPOINT=""
RUN CGO_ENABLED=1 GOOS=linux GOARCH=$TARGETARCH \
    go build -ldflags "-s -w -X 'loglynx/internal/telemetry.BuildEndpoint=${LOGLYNX_USAGE_TELEMETRY_ENDPOINT}'" -o /out/loglynx ./cmd/server


# Final image: small, secure runtime that still ships glibc for CGO
FROM gcr.io/distroless/base-debian12

# Create application directory and set as working dir so relative paths like
# `web/templates/**/*.html` and `geoip/*` resolve inside the container.
WORKDIR /app

# Copy binary from builder
COPY --from=builder /out/loglynx /usr/local/bin/loglynx

# Copy web assets (templates + static) so Gin can load templates from
# the expected relative path `web/templates/**/*.html`.
COPY --from=builder /src/web ./web

# Optional: create directories for volumes
VOLUME ["/data", "/app/geoip", "/traefik/logs"]

EXPOSE 8080


ENTRYPOINT ["/usr/local/bin/loglynx"]



