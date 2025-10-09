# Use official Go image as build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/web

# Final stage - minimal runtime image
FROM alpine:latest

# Install runtime dependencies (yt-dlp is available in Alpine repos!)
RUN apk add --no-cache \
    ca-certificates \
    yt-dlp \
    ffmpeg \
    wget

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/main .
COPY --from=builder /app/static ./static

# Create videos directory with proper permissions
RUN mkdir -p /app/videos && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port (configurable via environment variable)
EXPOSE 8591

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:${PORT:-8591}/ || exit 1

# Run the application
CMD ["./main"]
