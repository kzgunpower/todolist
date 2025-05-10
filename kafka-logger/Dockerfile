
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download -mod=readonly
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o kafka-logger .
FROM alpine:3.19
RUN apk add --no-cache \
    ca-certificates \
    tzdata && \
    update-ca-certificates
RUN adduser -D -u 1000 appuser
USER appuser
WORKDIR /app
COPY --from=builder --chown=appuser:appuser /app/kafka-logger .
RUN mkdir -p /app/logs && \
    touch /app/logs/logs.log && \
    chown appuser:appuser /app/logs/logs.log
VOLUME ["/app/logs"]


CMD ["./kafka-logger"]

