#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/web"

if ! command -v npm &> /dev/null; then
  echo "[ERROR] npm is not installed. Install Node.js (20+) and retry."
  exit 1
fi

if [ ! -d node_modules ]; then
  echo "[INFO] Installing web dependencies..."
  npm install
fi

if [ -f .env ]; then
  echo "[INFO] Using web/.env"
fi

npm run dev -- --host 0.0.0.0 --port 5173
