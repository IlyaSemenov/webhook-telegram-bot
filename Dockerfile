# Build stage
FROM golang:1.25-alpine3.22 AS builder

# The cmd/ directory to build
ARG BOT=bot

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
# Using the same build flags as in Makefile for consistency
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o bot \
    ./cmd/${BOT}

# Runner stage
FROM alpine:3.22 AS runner

# Install run deps
RUN apk add --no-cache ca-certificates curl

# Create non-root user for security
RUN adduser -u 10001 -S appuser

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/bot .

# Change ownership to non-root user
RUN chown -R appuser /app

# Switch to non-root user
USER appuser

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f "http://localhost:$PORT/health" || exit 1

# Run the application
CMD ["./bot"]
