FROM golang:1.25-alpine AS builder

# TARGETARCH is automatically set by Docker when using --platform
ARG TARGETARCH

WORKDIR /build

COPY pkg/config ./pkg/config

COPY apps/api-service ./apps/api-service

WORKDIR /build/apps/api-service

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -a -installsuffix cgo -o api-service ./cmd

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /root/
COPY --from=builder /build/apps/api-service/api-service .

EXPOSE 8080
CMD ["./api-service"]
