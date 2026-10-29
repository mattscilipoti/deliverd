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

# CGO off gives a static binary with no libc dependency; -s -w strips debug
# info. The build time (UTC, RFC 3339) is stamped in; the program shows it in
# the local time zone on the main screen and in help.
build_time="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" \
  go build -trimpath -ldflags="-s -w -X 'main.buildTime=${build_time}'" -o "$out" .

echo "built $out ($(du -h "$out" | cut -f1)) at ${build_time}"
