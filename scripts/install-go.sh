#!/usr/bin/env bash
# Install a Go toolchain for cerebro. Tries, in order:
#   1. scoop (no admin needed)
#   2. winget
#   3. choco
#   4. portable zip download into ~/.local/go
set -u

export PATH="$PATH:/c/Program Files/Go/bin:$HOME/go/bin:$HOME/scoop/shims:$HOME/scoop/apps/go/current/bin"

try_go() {
  if command -v go >/dev/null 2>&1; then
    echo "GO_AVAILABLE: $(go version) at $(command -v go)"
    return 0
  fi
  for p in "/c/Program Files/Go/bin/go.exe" "$HOME/scoop/apps/go/current/bin/go.exe" "$HOME/.local/go/go/bin/go.exe"; do
    if [ -x "$p" ]; then
      echo "GO_AVAILABLE: $($p version) at $p"
      return 0
    fi
  done
  return 1
}

if try_go; then exit 0; fi

echo "== step 1: scoop =="
if command -v scoop >/dev/null 2>&1; then
  scoop install go >/tmp/cerebro_scoop.log 2>&1 && echo SCOOP_OK || echo SCOOP_FAIL
  if try_go; then exit 0; fi
else
  echo "scoop not found"
fi

echo "== step 2: winget =="
if command -v winget >/dev/null 2>&1; then
  winget install --id GoLang.Go -e --accept-source-agreements --accept-package-agreements >/tmp/cerebro_winget.log 2>&1 && echo WINGET_OK || echo WINGET_FAIL
  if try_go; then exit 0; fi
else
  echo "winget not found"
fi

echo "== step 3: choco =="
if command -v choco >/dev/null 2>&1; then
  choco install golang -y >/tmp/cerebro_choco.log 2>&1 && echo CHOCO_OK || echo CHOCO_FAIL
  if try_go; then exit 0; fi
else
  echo "choco not found"
fi

echo "== step 4: portable zip =="
if command -v curl >/dev/null 2>&1; then
  ZVER=$(curl -s https://go.dev/VERSION?m=text | head -1)
  echo "latest Go version: $ZVER"
  ZURL="https://go.dev/dl/${ZVER}.windows-amd64.zip"
  echo "downloading $ZURL"
  curl -L -o /tmp/go.zip "$ZURL"
  mkdir -p "$HOME/.local"
  if command -v unzip >/dev/null 2>&1; then
    rm -rf "$HOME/.local/go"
    unzip -q /tmp/go.zip -d "$HOME/.local" && echo UNZIP_OK || echo UNZIP_FAIL
  elif command -v powershell >/dev/null 2>&1; then
    powershell -Command "Expand-Archive -Force /tmp/go.zip $HOME/.local" && echo PS_UNZIP_OK || echo PS_UNZIP_FAIL
  fi
  if try_go; then exit 0; fi
fi

echo "ALL_INSTALL_METHODS_FAILED"
echo "--- logs ---"
for f in /tmp/cerebro_scoop.log /tmp/cerebro_winget.log /tmp/cerebro_choco.log; do
  [ -f "$f" ] && { echo "### $f"; tail -20 "$f"; }
done
exit 1
