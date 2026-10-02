#!/usr/bin/env bash
# Builds minecx for Windows and Linux (amd64 + arm64) into ./dist
set -Eeuo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")"

OUT="dist"
mkdir -p "$OUT"

export CGO_ENABLED=0
LDFLAGS="-s -w"

build() {
  local goos="$1" goarch="$2" name="$3"
  echo "==> $goos/$goarch -> $OUT/$name"
  GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags "$LDFLAGS" -o "$OUT/$name" .
}

build windows amd64 "minecx-windows-amd64.exe"
build linux   amd64 "minecx-linux-amd64"
build linux   arm64 "minecx-linux-arm64"

echo
echo "Built:"
ls -lh "$OUT"
