#!/bin/bash

set -e

# Default values
SERVER_ADDR=""
TOKEN=""
VERSION="latest"
REPO="awandataindonesia/cekping-agent"
SECURE="false"

# Parse arguments
while getopts "b:t:s:v:S" opt; do
  case $opt in
    b) BRANCH="$OPTARG"
    ;;
    t) TOKEN="$OPTARG"
    ;;
    s) SERVER_ADDR="$OPTARG"
    ;;
    v) VERSION="$OPTARG"
    ;;
    S) SECURE="true"
    ;;
    \?) echo "Invalid option -$OPTARG" >&2
    exit 1
    ;;
  esac
done

if [ "$VERSION" == "latest" ] && [ ! -z "$BRANCH" ] && [ "$BRANCH" != "main" ]; then
    VERSION="$BRANCH"
fi

if [ -z "$TOKEN" ]; then
    echo "Error: Token is required. Use -t <token>"
    exit 1
fi

if [ -z "$SERVER_ADDR" ]; then
    echo "Error: Server address is required. Use -s <host:port>"
    exit 1
fi

# Detect OS and Arch
OS="linux"
ARCH="$(uname -m)"

if [ "$ARCH" == "x86_64" ]; then
    ARCH="amd64"
elif [ "$ARCH" == "aarch64" ]; then
    ARCH="arm64"
else
    echo "Unsupported architecture: $ARCH"
    exit 1
fi

BINARY_NAME="cekping-agent-${OS}-${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/${VERSION}/download/${BINARY_NAME}"
if [ "$VERSION" == "latest" ]; then
    DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${BINARY_NAME}"
fi

# Stop existing service if running
if systemctl is-active --quiet cekping-agent; then
    echo "Stopping existing CekPing Agent service..."
    systemctl stop cekping-agent
fi

echo "Downloading CekPing Agent from $DOWNLOAD_URL..."
if ! curl -f -L -o /usr/local/bin/cekping-agent "$DOWNLOAD_URL"; then
    echo "Error: Failed to download agent binary from $DOWNLOAD_URL"
    echo "Please ensure the version '$VERSION' exists and assets are uploaded."
    exit 1
fi
chmod +x /usr/local/bin/cekping-agent

# Create Systemd Service
echo "Creating systemd service..."
cat <<EOF > /etc/systemd/system/cekping-agent.service
[Unit]
Description=CekPing Agent
After=network.target

[Service]
ExecStartPre=/usr/bin/curl -f -s -L -o /usr/local/bin/cekping-agent "${DOWNLOAD_URL}"
ExecStartPre=/bin/chmod +x /usr/local/bin/cekping-agent
ExecStart=/usr/local/bin/cekping-agent
Restart=always
User=root
Environment=CEKPING_TOKEN=${TOKEN}
Environment=CEKPING_SERVER=${SERVER_ADDR}
Environment=CEKPING_SECURE=${SECURE}

[Install]
WantedBy=multi-user.target
EOF

# Reload and Start
systemctl daemon-reload
systemctl enable --now cekping-agent

echo "CekPing Agent installed and started successfully!"
echo "Connected to: $SERVER_ADDR"
