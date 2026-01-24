# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary
RUN CGO_ENABLED=1 GOOS=linux go build -o atomicbase .

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /app/atomicbase .

# Create data directory
RUN mkdir -p /app/atomicdata

# Expose port
EXPOSE 8080

# Set default environment
ENV PORT=:8080
ENV DB_PATH=/app/atomicdata/tenants.db
ENV DATA_DIR=/app/atomicdata

# Run
CMD ["./atomicbase"]
