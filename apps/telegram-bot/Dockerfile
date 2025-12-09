FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY pkg/config ./pkg/config

COPY apps/telegram-bot ./apps/telegram-bot

WORKDIR /build/apps/telegram-bot

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o telegram-bot ./cmd

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /root/
COPY --from=builder /build/apps/telegram-bot/telegram-bot .

CMD ["./telegram-bot"]
