# Telegram Bot Service - Golang
# Placeholder Dockerfile for Go service
FROM golang:1.25 AS builder

WORKDIR /build
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o telegram-bot ./cmd

FROM ubuntu:24.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    unzip \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /root/
COPY --from=builder /build/telegram-bot .

CMD ["./telegram-bot"]
