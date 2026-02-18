# Build Stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git

# Copy gomod
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build Binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o pingve-agent ./cmd/agent

# Runtime Stage
FROM alpine:latest

WORKDIR /app

# Install CA Certs for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/pingve-agent .

# Default Envs
ENV PINGVE_SERVER=""
ENV PINGVE_TOKEN=""

# Entrypoint
ENTRYPOINT ["./pingve-agent"]
