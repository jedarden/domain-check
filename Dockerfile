# Stage 1: Build the Go binary
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /build

# Copy go.mod and go.sum first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with optimizations
# CGO_ENABLED=0: Static binary, no libc dependency
# -ldflags: Strip debug info and DWARF for smaller binary
# -trimpath: Remove file system paths from binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags='-s -w -extldflags "-static"' \
    -trimpath \
    -o domain-check \
    ./cmd/domain-check

# Stage 2: Minimal runtime image
FROM alpine:3.19

# Install runtime dependencies
# ca-certificates: TLS/HTTPS support
# tzdata: Timezone data
# wget: Health check support
RUN apk add --no-cache ca-certificates tzdata wget

# Create non-root user for security
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D appuser

WORKDIR /app

# Copy only the binary from builder
COPY --from=builder /build/domain-check /app/domain-check

# Set ownership to non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose the HTTP port
EXPOSE 8080

# Health check endpoint
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

# Run the server
CMD ["/app/domain-check", "serve"]
