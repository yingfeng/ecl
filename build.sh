#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")"

BINARY="llmwiki-server"
SRC_DIR="./backend"
TARGET_DIR="./bin"

echo "==> Downloading dependencies..."
cd "$SRC_DIR"
go mod download

echo "==> Building ${BINARY}..."
mkdir -p "../${TARGET_DIR}"
CGO_ENABLED=0 go build -o "../${TARGET_DIR}/${BINARY}" -ldflags="-s -w" .

echo "==> Build complete: ${TARGET_DIR}/${BINARY}"
ls -lh "../${TARGET_DIR}/${BINARY}"
