#!/usr/bin/env bash
# Build a static deliverd executable for Linux (Debian and other distros).
#
# Usage: ./build.sh [amd64|arm64]     (default: amd64)
# Output: dist/deliverd-linux-<arch>
set -euo pipefail

arch="${1:-amd64}"
case "$arch" in
  amd64|arm64) ;;
  *) echo "usage: $0 [amd64|arm64]" >&2; exit 2 ;;
esac

cd "$(dirname "$0")"
out="dist/deliverd-linux-${arch}"
mkdir -p dist

# CGO off gives a static binary with no libc dependency; -s -w strips debug info.
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" \
  go build -trimpath -ldflags="-s -w" -o "$out" .

echo "built $out ($(du -h "$out" | cut -f1))"
