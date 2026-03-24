#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_DIR="${HOME}/.local/bin"

mkdir -p "$TARGET_DIR"

for tool in codex-usage claude-usage llm-usage; do
  cp "$SCRIPT_DIR/${tool}.py" "$TARGET_DIR/$tool"
  chmod +x "$TARGET_DIR/$tool"
  printf 'Installed %s\n' "$TARGET_DIR/$tool"
done

printf 'Run: llm-usage\n'
