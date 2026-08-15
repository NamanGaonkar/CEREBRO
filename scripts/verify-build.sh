#!/usr/bin/env bash
set -u
export PATH="$PATH:/c/Users/Naman Gaonkar/scoop/shims"

echo "=== anacrolix AddTorrent/AddMagnet ==="
rg -n -A1 "func \(cl \*Client\) (AddTorrent|AddMagnet)\(" ~/go/pkg/mod/github.com/anacrolix/torrent@*/ -g '*.go' 2>/dev/null | head -20

echo "=== kkdai SearchResult struct ==="
rg -n -B1 -A10 "type SearchResult struct" ~/go/pkg/mod/github.com/kkdai/youtube/v2@*/ -g '*.go' 2>/dev/null | head -25

echo "=== kkdai Video struct ==="
rg -n -A32 "type Video struct" ~/go/pkg/mod/github.com/kkdai/youtube/v2@*/ -g '*.go' 2>/dev/null | head -42

echo "=== kkdai client methods ==="
rg -n "func \(c \*Client\) (Search|GetVideo|GetStream)\(" ~/go/pkg/mod/github.com/kkdai/youtube/v2@*/ -g '*.go' 2>/dev/null | head -10

echo "=== bubbles progress options ==="
rg -n "func With(DefaultGradient|Width)" ~/go/pkg/mod/github.com/charmbracelet/bubbles@v1.0.0/progress/ -g '*.go' 2>/dev/null | head -5

echo "=== bubbletea Tick ==="
rg -n "func Tick" ~/go/pkg/mod/github.com/charmbracelet/bubbletea@v1.3.10/ -g '*.go' 2>/dev/null | head -5

echo "=== BUILD ==="
go build ./... 2>&1 | head -80
echo "BUILD_EXIT=${PIPESTATUS[0]}"

echo "=== VET ==="
go vet ./... 2>&1 | head -40
echo "VET_EXIT=${PIPESTATUS[0]}"
