#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPOSITORY_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

cd "${REPOSITORY_ROOT}"
mkdir -p dist

CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
    go build \
    -buildvcs=false \
    -trimpath \
    -ldflags="-s -w" \
    -o dist/termchat-darwin-arm64 \
    ./cmd/client

printf 'Built Apple Silicon client: %s\n' "${REPOSITORY_ROOT}/dist/termchat-darwin-arm64"
