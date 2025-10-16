# Multi-stage build for Shinzo Network Indexer
# Stage 1: Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the applications
RUN make build
RUN make build-catch-up

# Stage 2: Runtime stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata curl

# Create non-root user for security
RUN addgroup -g 1001 -S shinzo && \
    adduser -u 1001 -S shinzo -G shinzo

# Create necessary directories
RUN mkdir -p /app/bin /app/logs /app/data /app/config && \
    chown -R shinzo:shinzo /app

# Copy binaries from builder stage
COPY --from=builder /app/bin/block_poster /app/bin/
COPY --from=builder /app/bin/catch_up /app/bin/

# Copy configuration files
COPY --from=builder /app/scripts/ /app/scripts/

# Set working directory
WORKDIR /app

# Switch to non-root user
USER shinzo

# Expose DefraDB port (if running embedded)
EXPOSE 9181

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD curl -f http://localhost:9181/api/v0/graphql || exit 1

# Default command (can be overridden)
CMD ["/app/bin/block_poster"]
