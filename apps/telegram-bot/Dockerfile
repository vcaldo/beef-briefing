FROM golang:1.25 AS builder

WORKDIR /build

COPY pkg/config ./pkg/config

COPY apps/telegram-bot ./apps/telegram-bot

WORKDIR /build/apps/telegram-bot

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o telegram-bot ./cmd

FROM ubuntu:24.04

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /root/
COPY --from=builder /build/apps/telegram-bot/telegram-bot .

CMD ["./telegram-bot"]
