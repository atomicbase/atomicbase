# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies for CGO (required for SQLite FTS5)
RUN apk add --no-cache gcc musl-dev

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with CGO enabled and FTS5 tag for full-text search
RUN CGO_ENABLED=1 GOOS=linux go build -tags fts5 -o atomicbase .

# Runtime stage
FROM alpine:3.20

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /app/atomicbase .

# Create data directory (will be replaced by volume mount on Fly.io)
RUN mkdir -p /data

# Expose port
EXPOSE 8080

# Set default environment variables
ENV PORT=:8080
ENV DB_PATH=/data/tenants.db
ENV DATA_DIR=/data

# Run the server
CMD ["./atomicbase"]
