# Build stage
FROM golang:1.25 AS builder
WORKDIR /app

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o legacy-export-generator ./cmd

# Runtime stage
FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/legacy-export-generator .
ENTRYPOINT ["./legacy-export-generator"]
