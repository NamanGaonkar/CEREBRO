#!/usr/bin/env bash
echo "=== repo visibility ==="
curl -s https://api.github.com/repos/NamanGaonkar/CEREBRO | grep -E '"full_name"|"private"|"default_branch"' | head -3

echo "=== files at repo root ==="
curl -s "https://api.github.com/repos/NamanGaonkar/CEREBRO/contents/" | grep '"name"' | head -40

echo "=== full recursive tree (looking for junk: exe/zip/downloads) ==="
curl -s "https://api.github.com/repos/NamanGaonkar/CEREBRO/git/trees/main?recursive=1" | grep '"path"' | head -60
