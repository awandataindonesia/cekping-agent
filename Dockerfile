FROM alpine:latest

WORKDIR /app

# Install CA Certs for HTTPS
RUN apk --no-cache add ca-certificates

# We use the pre-built binaries from the build-binaries job
# TARGETARCH is automatically provided by Docker Buildx (amd64, arm64, etc.)
ARG TARGETARCH
COPY dist/cekping-agent-linux-${TARGETARCH} ./cekping-agent

# Ensure the binary is executable
RUN chmod +x ./cekping-agent

# Default Envs
ENV CEKPING_SERVER=""
ENV CEKPING_TOKEN=""

# Entrypoint
ENTRYPOINT ["./cekping-agent"]
