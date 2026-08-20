# Multi-stage build for Guardians of the Lake backend (pure-Go SQLite, no CGO required)
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/server

# Minimal runtime image
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/server /app/server
COPY --from=builder /app/migrations /app/migrations

# Create uploads directory
RUN mkdir -p /app/uploads /app/data

ENV PORT=8080
ENV DB_PATH=/app/data/guardians.db
ENV MIGRATIONS_DIR=/app/migrations
ENV UPLOADS_DIR=/app/uploads
ENV STATIC_DIR=/app/frontend

EXPOSE 8080

CMD ["/app/server"]
