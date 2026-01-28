#!/bin/bash
# Run the Streamlit UI for the Trading Bot

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Change to the project directory
cd "$SCRIPT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check if uv is installed
if ! command -v uv &> /dev/null; then
    log_error "uv is not installed."
    log_info "Install it with: curl -LsSf https://astral.sh/uv/install.sh | sh"
    exit 1
fi

# Check if virtual environment exists, create if not
VENV_DIR="$SCRIPT_DIR/.venv"
if [ ! -d "$VENV_DIR" ]; then
    log_info "Creating virtual environment with uv..."
    uv venv
    log_success "Virtual environment created"
    
    log_info "Installing dependencies..."
    uv sync
    log_success "Dependencies installed"
fi

# Set Python path
export PYTHONPATH="$SCRIPT_DIR:$PYTHONPATH"

# Pick an available port (default 8501, then 8502-8510)
PORT=$(uv run python - <<'PY'
import socket
import sys

for port in range(8501, 8511):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        try:
            sock.bind(("127.0.0.1", port))
        except OSError:
            continue
        print(port)
        sys.exit(0)

print("")
sys.exit(1)
PY
)

if [ -z "$PORT" ]; then
    log_error "No free port found in range 8501-8510."
    exit 1
fi

# Run the Streamlit app using uv
echo "🚀 Starting Cobalt Trading Bot UI..."
echo "   URL: http://localhost:${PORT}"
echo ""

uv run streamlit run src/ui/main.py --server.port "$PORT"
