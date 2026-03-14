# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o service-monitor ./cmd/server

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/service-monitor .
COPY --from=builder /app/internal/web/templates ./internal/web/templates

RUN addgroup -S monitor && adduser -S monitor -G monitor
USER monitor

EXPOSE 3001

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:3001/healthz || exit 1

ENV LISTEN_ADDR=":3001" \
    DB_PATH="/app/data/service-monitor.db" \
    DATA_DIR="/app/data"

VOLUME ["/app/data"]

ENTRYPOINT ["./service-monitor"]
