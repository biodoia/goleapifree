#!/bin/bash

# GoLeapAI Web UI Launcher
# Code Page 437 Edition

set -e

# Colors
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# ASCII Art
cat << "EOF"
╔═══════════════════════════════════════════════════════════════════╗
║                     █▀▀ █▀█ █   █▀▀ ▄▀█ █▀█ ▄▀█ █                ║
║                     █▄█ █▄█ █▄▄ ██▄ █▀█ █▀▀ █▀█ █                ║
║                  FREE LLM GATEWAY - CP437 WEB UI                  ║
╚═══════════════════════════════════════════════════════════════════╝
EOF

echo ""

# Check if templ is installed
if ! command -v templ &> /dev/null; then
    echo -e "${YELLOW}[WARN]${NC} templ not found. Installing..."
    go install github.com/a-h/templ/cmd/templ@latest
    export PATH=$PATH:~/go/bin
fi

# Generate templates
echo -e "${CYAN}[INFO]${NC} Generating Templ templates..."
templ generate

if [ $? -eq 0 ]; then
    echo -e "${GREEN}[OK]${NC} Templates generated successfully"
else
    echo -e "${RED}[ERROR]${NC} Failed to generate templates"
    exit 1
fi

# Create data directory if not exists
mkdir -p ./data

# Run Web UI
PORT=${PORT:-8080}
CONFIG=${CONFIG:-./configs/webui.yaml}

echo -e "${CYAN}[INFO]${NC} Starting GoLeapAI Web UI..."
echo -e "${CYAN}[INFO]${NC} Port: ${PORT}"
echo -e "${CYAN}[INFO]${NC} Config: ${CONFIG}"
echo ""

# Build and run
go run cmd/webui/main.go --port $PORT --config $CONFIG
