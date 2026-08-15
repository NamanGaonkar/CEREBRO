#!/usr/bin/env bash
# ============================================================
#  CEREBRO - build script (macOS / Linux)
#  Produces a standalone binary with no shared-library deps.
#  Requires: Go 1.22+  (brew install go / apt install golang)
#
#  Usage:
#    ./build.sh            # native build
#    GOOS=windows ./build.sh   # cross-compile to Windows
# ============================================================
set -euo pipefail

if ! command -v go >/dev/null 2>&1; then
  echo "[ERROR] Go is not installed or not on PATH."
  echo "        macOS: brew install go"
  echo "        Linux: sudo apt install golang"
  exit 1
fi

GOOS="${GOOS:-$(go env GOOS)}"
GOARCH="${GOARCH:-$(go env GOARCH)}"
OUT="cerebro"
if [ "$GOOS" = "windows" ]; then
  OUT="cerebro.exe"
fi

echo "[1/2] Fetching dependencies..."
go mod tidy

echo "[2/2] Building $OUT ($GOOS/$GOARCH) ..."
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$OUT" ./cmd/app

echo "Done! Run it with:  ./$OUT"
