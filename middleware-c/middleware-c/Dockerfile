# Multi-stage build for middleware-c (base64 inline version)
# Stage 1: Build
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git and CA certificates for Go module download (if needed)
RUN apk add --no-cache git ca-certificates && update-ca-certificates

# Copy go module files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build static binary for Linux
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /middleware-c main.go

# Stage 2: Runtime
FROM alpine:3.20

WORKDIR /app

# CA certificates in case upstream uses HTTPS
RUN apk add --no-cache ca-certificates && update-ca-certificates

# Copy binary
COPY --from=builder /middleware-c /app/middleware-c

# Copy default config (can be overridden by mounting/ENV)
COPY config.json.example /app/config.json

# Listen port (match listen_http in config.json)
EXPOSE 8081

# Allow override of config path via ENV if needed
ENV CONFIG_PATH=/app/config.json

ENTRYPOINT ["/app/middleware-c"]
CMD ["-config", "/app/config.json"]
