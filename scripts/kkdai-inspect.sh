#!/usr/bin/env bash
set -u
K=~/go/pkg/mod/github.com/kkdai/youtube/v2@v2.10.6

echo "=== files in module ==="
ls "$K" | head -40

echo "=== Search-related funcs ==="
rg -n "func .*Search" "$K" -g '*.go' | head -20

echo "=== Format struct ==="
rg -n -A35 "type Format struct" "$K" -g '*.go' | head -45

echo "=== FormatList methods ==="
rg -n "func (f FormatList)" "$K" -g '*.go' | head -20

echo "=== Client methods (all) ==="
rg -n "func \(c \*Client\)" "$K" -g '*.go' | head -40

echo "=== SearchResult type (any casing) ==="
rg -n -i "searchresult|search result" "$K" -g '*.go' | head -15
