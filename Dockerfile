# Multi-stage Dockerfile for LogLynx
# Builder stage: compiles a static linux/amd64 binary
FROM golang:1.25.5 AS builder

WORKDIR /src

# Copy go.mod and go.sum first to leverage Docker layer cache
COPY go.mod go.sum ./
RUN go mod download

# Copy rest of the sources
COPY . .

# Build the server binary (CGO enabled for sqlite/geoip native deps)
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -ldflags "-s -w" -o /out/loglynx ./cmd/server


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

# Run as non-root user for security
USER 65532

ENTRYPOINT ["/usr/local/bin/loglynx"]



