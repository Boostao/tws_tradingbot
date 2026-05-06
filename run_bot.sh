#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if [[ ! -d .venv ]]; then
	echo "Virtual environment not found at $SCRIPT_DIR/.venv" >&2
	exit 1
fi

source .venv/bin/activate
export PYTHONPATH="$SCRIPT_DIR:${PYTHONPATH:-}"

python -m src.bot.live_runner "$@"