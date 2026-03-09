#!/bin/bash

set -e

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"

if [ "$OS" == "linux" ]; then
    SCRIPT_NAME="install-linux.sh"
else
    echo "Unsupported OS: $OS"
    exit 1
fi

BRANCH="main"

for arg in "$@"; do
    if [[ "$prev" == "-b" ]]; then
        BRANCH="$arg"
    fi
    prev="$arg"
done

DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
if [ -f "$DIR/$SCRIPT_NAME" ]; then
    echo "Detected OS: $OS. Running local script $SCRIPT_NAME..."
    bash "$DIR/$SCRIPT_NAME" "$@"
else
    SCRIPT_URL="https://raw.githubusercontent.com/awandataindonesia/cekping-agent/${BRANCH}/scripts/$SCRIPT_NAME"
    echo "Detected OS: $OS. Downloading and running $SCRIPT_URL..."
    curl -sL "$SCRIPT_URL" | bash -s -- "$@"
fi
