#!/bin/bash

set -e

# Default values
SERVER_ADDR=""
TOKEN=""
VERSION="latest"
REPO="awandataindonesia/cekping-agent"
SECURE="false"

# Parse arguments
while getopts "t:s:v:S" opt; do
  case $opt in
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

if [ -z "$TOKEN" ]; then
    echo "Error: Token is required. Use -t <token>"
    exit 1
fi

if [ -z "$SERVER_ADDR" ]; then
    echo "Error: Server address is required. Use -s <host:port>"
    exit 1
fi

# Detect OS and Arch
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

if [ "$ARCH" == "x86_64" ]; then
    ARCH="amd64"
elif [ "$ARCH" == "aarch64" ]; then
    ARCH="arm64"
else
    echo "Unsupported architecture: $ARCH"
    exit 1
fi

BINARY_NAME="pingve-agent-${OS}-${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/${VERSION}/download/${BINARY_NAME}"
if [ "$VERSION" == "latest" ]; then
    DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${BINARY_NAME}"
fi

echo "Downloading CekPing Agent from $DOWNLOAD_URL..."
if ! curl -f -L -o /usr/local/bin/pingve-agent "$DOWNLOAD_URL"; then
    echo "Error: Failed to download agent binary from $DOWNLOAD_URL"
    echo "Please ensure the version '$VERSION' exists and assets are uploaded."
    exit 1
fi
chmod +x /usr/local/bin/pingve-agent

# Create Systemd Service
echo "Creating systemd service..."
cat <<EOF > /etc/systemd/system/pingve-agent.service
[Unit]
Description=PingVe Agent
After=network.target

[Service]
ExecStart=/usr/local/bin/pingve-agent
Restart=always
User=root
Environment=PINGVE_TOKEN=${TOKEN}
Environment=PINGVE_SERVER=${SERVER_ADDR}
Environment=PINGVE_SECURE=${SECURE}

[Install]
WantedBy=multi-user.target
EOF

# Reload and Start
systemctl daemon-reload
systemctl enable --now pingve-agent

echo "CekPing Agent installed and started successfully!"
echo "Connected to: $SERVER_ADDR"
