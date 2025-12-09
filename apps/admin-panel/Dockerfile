# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install templ CLI
RUN go install github.com/a-h/templ/cmd/templ@latest

# Copy go mod files first for caching
COPY apps/admin-panel/go.mod apps/admin-panel/go.sum ./
RUN go mod download

# Copy source code
COPY apps/admin-panel/ .

# Generate templ files
RUN templ generate

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary and static files
COPY --from=builder /app/main .
COPY --from=builder /app/static ./static

EXPOSE 8081

CMD ["./main"]
