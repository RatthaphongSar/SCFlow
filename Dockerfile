# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies for CGO (SQLite)
RUN apk add --no-cache gcc musl-dev

# Copy go mod files first (cache layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary with CGO enabled for SQLite
RUN CGO_ENABLED=1 GOOS=linux go build -o scflow ./cmd/server

# Runtime stage (small image)
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/scflow .

# Copy views and public assets
COPY --from=builder /app/views ./views
COPY --from=builder /app/public ./public

# Create uploads directory and data directory
RUN mkdir -p ./public/uploads ./data

# Expose port
EXPOSE 3000

# Default env
ENV PORT=3000
ENV DB_DSN=/app/data/scflow.db

# Run
CMD ["./scflow"]
