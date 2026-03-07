FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git

# Copy gomod
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build Binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=$TARGETARCH go build -ldflags="-s -w" -o cekping-agent ./cmd/agent

# Runtime Stage
FROM alpine:latest

WORKDIR /app

# Install CA Certs for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/cekping-agent .

# Default Envs
ENV CEKPING_SERVER=""
ENV CEKPING_TOKEN=""

# Entrypoint
ENTRYPOINT ["./cekping-agent"]
