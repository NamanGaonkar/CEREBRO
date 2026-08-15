#!/usr/bin/env bash
# ============================================================
#  CEREBRO - one-line installer (macOS / Linux)
#
#  Usage:
#    curl -fsSL https://raw.githubusercontent.com/NamanGaonkar/CEREBRO/main/install.sh | bash
#
#  Downloads the latest prebuilt binary from GitHub Releases.
#  If no release exists yet, falls back to building from source
#  (requires Git + Go). Installs to ~/.cerebro/bin and adds it
#  to your PATH.
# ============================================================
set -euo pipefail

OWNER="NamanGaonkar"
REPO="CEREBRO"
VERSION="latest"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64 | amd64) ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
esac

if [ "$OS" = "darwin" ]; then
  PLATFORM="darwin-${ARCH}"
else
  PLATFORM="${OS}-${ARCH}"
fi

INSTALL_DIR="${CEREBRO_HOME:-$HOME/.cerebro/bin}"
mkdir -p "$INSTALL_DIR"

# --- try downloading a prebuilt release ---
DOWNLOADED=0
if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/$OWNER/$REPO/releases/latest" 2>/dev/null \
    | grep -m1 '"tag_name"' \
    | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/' || true)"
fi

if [ -n "$VERSION" ]; then
  URL="https://github.com/$OWNER/$REPO/releases/download/$VERSION/cerebro-${PLATFORM}.tar.gz"
  echo "Downloading CEREBRO $VERSION ($PLATFORM) ..."
  TMP="$(mktemp -d)"
  trap 'rm -rf "$TMP"' EXIT
  if curl -fsSL "$URL" -o "$TMP/cerebro.tar.gz"; then
    tar -xzf "$TMP/cerebro.tar.gz" -C "$INSTALL_DIR"
    DOWNLOADED=1
  else
    echo "No prebuilt release for $PLATFORM yet." >&2
  fi
fi

# --- fallback: build from source ---
if [ "$DOWNLOADED" -eq 0 ]; then
  if ! command -v git >/dev/null 2>&1 || ! command -v go >/dev/null 2>&1; then
    echo "[ERROR] No GitHub Release for CEREBRO yet AND Git/Go are not installed." >&2
    echo "        Either create a release (git tag v1.0.0; git push origin v1.0.0)" >&2
    echo "        or install Git and Go and retry." >&2
    exit 1
  fi
  echo "Building CEREBRO from source ..."
  TMP="$(mktemp -d)"
  trap 'rm -rf "$TMP"' EXIT
  git clone --depth 1 "https://github.com/$OWNER/$REPO.git" "$TMP/src"
  ( cd "$TMP/src" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$INSTALL_DIR/cerebro" ./cmd/app )
fi
chmod +x "$INSTALL_DIR/cerebro"

VER="$("$INSTALL_DIR/cerebro" --version 2>/dev/null || echo "CEREBRO $VERSION")"

case "$SHELL" in
  *zsh*) PROFILE="$HOME/.zshrc" ;;
  *bash*) PROFILE="$HOME/.bashrc" ;;
  *) PROFILE="$HOME/.profile" ;;
esac
PATH_NOTE=""
if ! grep -qF "$INSTALL_DIR" "$PROFILE" 2>/dev/null; then
  echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$PROFILE"
  PATH_NOTE="PATH updated in $PROFILE - open a NEW terminal, then run:"
fi

echo ""
echo "  $VER"
echo "  CEREBRO installed to: $INSTALL_DIR"
if [ -n "$PATH_NOTE" ]; then
  echo "  $PATH_NOTE"
fi
echo "  Run:  cerebro"
echo "  Built by Naman Gaonkar - https://naman-gaonkar.vercel.app/"
