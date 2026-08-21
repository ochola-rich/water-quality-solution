# Production image used by Render. Its build context is the repository root so
# it can package both the Go API and the static frontend it serves.
FROM golang:1.26-alpine AS builder

WORKDIR /build/backend

RUN apk add --no-cache ca-certificates git

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/guardians-server ./cmd/server

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
  && mkdir -p /var/data/uploads

COPY --from=builder /out/guardians-server /app/guardians-server
COPY --from=builder /build/backend/migrations /app/migrations
COPY frontend /app/frontend

ENV PORT=3000 \
    DB_PATH=/var/data/guardians.db \
    MIGRATIONS_DIR=/app/migrations \
    UPLOADS_DIR=/var/data/uploads \
    STATIC_DIR=/app/frontend \
    AUTO_MIGRATE=true

EXPOSE 3000

CMD ["/app/guardians-server"]
