#!/usr/bin/env bash
# Builds minecx for Windows (console + silent GUI) and Linux (amd64 + arm64) into ./dist
set -Eeuo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")"

OUT="dist"
mkdir -p "$OUT"

export CGO_ENABLED=0

build() {
  local goos="$1" goarch="$2" name="$3" subsystem="${4:-}"
  local flags="-s -w"
  [[ -n "$subsystem" ]] && flags="$flags -H=$subsystem"
  echo "==> $goos/$goarch $subsystem -> $OUT/$name"
  GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags "$flags" -o "$OUT/$name" .
}

build windows amd64 "minecx-windows-amd64.exe" ""
build windows amd64 "minecx-windows-amd64-gui.exe" "windowsgui"
build linux   amd64 "minecx-linux-amd64" ""
build linux   arm64 "minecx-linux-arm64" ""

echo
echo "Built:"
ls -lh "$OUT"
