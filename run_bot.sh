#!/usr/bin/env bash
#
# run_bot.sh - Launch the TWS TraderBot live trading runner
#
# This script:
#   1. Uses uv to manage dependencies and virtual environment
#   2. Checks if TWS/IB Gateway is running
#   3. Launches the live trading bot
#
# Usage:
#   ./run_bot.sh                    # Run with default settings
#   ./run_bot.sh --check            # Only check prerequisites
#   ./run_bot.sh --verbose          # Run with verbose logging
#   ./run_bot.sh --config FILE      # Use custom config file
#   ./run_bot.sh --strategy FILE    # Use custom strategy file
#

set -e

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color


# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Banner
echo ""
echo "============================================================"
echo "        TWS TRADERBOT - Live Trading Runner"
echo "============================================================"
echo ""
log_info "Container runs: podman-compose -f docker-compose.prod.yml up -d bot"

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
else
    log_success "Virtual environment found"
fi

# Check Python version via uv
PYTHON_VERSION=$(uv run python --version 2>&1 | cut -d' ' -f2)
log_info "Python version: $PYTHON_VERSION"

# Check if required packages are installed
log_info "Checking dependencies..."
if uv run python -c "import pandas, numpy, yaml" 2>/dev/null; then
    log_success "Core dependencies available"
else
    log_warn "Missing dependencies. Running: uv sync"
    uv sync
fi

# Check Nautilus Trader (optional but recommended)
if uv run python -c "import nautilus_trader" 2>/dev/null; then
    log_success "Nautilus Trader available"
else
    log_warn "Nautilus Trader not installed. Bot will run in simulation mode."
    log_info "Install with: uv sync"
fi

# Check TWS/IB Gateway connection
log_info "Checking TWS/IB Gateway connection..."

# Default ports
TWS_PAPER_PORT=7497
TWS_LIVE_PORT=7496
GATEWAY_PAPER_PORT=4002
GATEWAY_LIVE_PORT=4001

# Try to detect IB connection
check_port() {
    local port=$1
    if nc -z -w2 127.0.0.1 $port 2>/dev/null; then
        return 0
    else
        return 1
    fi
}

IB_CONNECTED=false
IB_PORT=""
IB_MODE=""

if check_port $TWS_PAPER_PORT; then
    IB_CONNECTED=true
    IB_PORT=$TWS_PAPER_PORT
    IB_MODE="TWS Paper"
elif check_port $TWS_LIVE_PORT; then
    IB_CONNECTED=true
    IB_PORT=$TWS_LIVE_PORT
    IB_MODE="TWS Live"
elif check_port $GATEWAY_PAPER_PORT; then
    IB_CONNECTED=true
    IB_PORT=$GATEWAY_PAPER_PORT
    IB_MODE="Gateway Paper"
elif check_port $GATEWAY_LIVE_PORT; then
    IB_CONNECTED=true
    IB_PORT=$GATEWAY_LIVE_PORT
    IB_MODE="Gateway Live"
fi

if [ "$IB_CONNECTED" = true ]; then
    log_success "IB connection found: $IB_MODE (port $IB_PORT)"
else
    log_warn "No IB connection detected on standard ports"
    log_info "Standard ports: TWS Paper=$TWS_PAPER_PORT, TWS Live=$TWS_LIVE_PORT"
    log_info "                Gateway Paper=$GATEWAY_PAPER_PORT, Gateway Live=$GATEWAY_LIVE_PORT"
    log_info ""
    log_info "Please start TWS or IB Gateway and enable API connections:"
    log_info "  TWS: File → Global Configuration → API → Settings"
    log_info "  Enable: 'Enable ActiveX and Socket Clients'"
    log_info "  Add localhost to 'Trusted IPs'"
    log_info ""
    
    # Ask user if they want to continue anyway (simulation mode)
    read -p "Continue in simulation mode? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Exiting. Start TWS/Gateway and try again."
        exit 0
    fi
fi

echo ""
log_info "Starting live trading runner..."
echo ""

# Set PYTHONPATH
export PYTHONPATH="$SCRIPT_DIR:$PYTHONPATH"

# Run the bot with all passed arguments using uv
uv run python -m src.bot.live_runner "$@"

# Capture exit code
EXIT_CODE=$?

echo ""
if [ $EXIT_CODE -eq 0 ]; then
    log_success "Bot exited normally"
else
    log_error "Bot exited with code $EXIT_CODE"
fi

exit $EXIT_CODE
