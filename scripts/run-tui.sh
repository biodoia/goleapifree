#!/bin/bash
#
# GoLeapAI TUI Launcher
# Avvia la Terminal User Interface per GoLeapAI Gateway
#

set -e

# Colors
CYAN='\033[0;36m'
PINK='\033[0;95m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Banner
echo -e "${PINK}"
cat << "EOF"
  ▄████  ▒█████   ██▓    ▓█████ ▄▄▄       ██▓███
 ██▒ ▀█▒▒██▒  ██▒▓██▒    ▓█   ▀▒████▄    ▓██░  ██▒
▒██░▄▄▄░▒██░  ██▒▒██░    ▒███  ▒██  ▀█▄  ▓██░ ██▓▒
░▓█  ██▓▒██   ██░▒██░    ▒▓█  ▄░██▄▄▄▄██ ▒██▄█▓▒ ▒
░▒▓███▀▒░ ████▓▒░░██████▒░▒████▒▓█   ▓██▒▒██▒ ░  ░
 ░▒   ▒ ░ ▒░▒░▒░ ░ ▒░▓  ░░░ ▒░ ░▒▒   ▓▒█░▒▓▒░ ░  ░
  ░   ░   ░ ▒ ▒░ ░ ░ ▒  ░ ░ ░  ░ ▒   ▒▒ ░░▒ ░
░ ░   ░ ░ ░ ░ ▒    ░ ░      ░    ░   ▒   ░░
      ░     ░ ░      ░  ░   ░  ░     ░  ░

EOF
echo -e "${CYAN}GoLeapAI Gateway - Cyberpunk TUI${NC}"
echo ""

# Check if binary exists
TUI_BIN="./bin/goleapai-tui"

if [ ! -f "$TUI_BIN" ]; then
    echo -e "${YELLOW}TUI binary not found. Building...${NC}"
    go build -o bin/goleapai-tui ./cmd/tui/
    echo -e "${GREEN}✓ Build complete${NC}"
fi

# Check database
if [ ! -f "./data/goleapai.db" ]; then
    echo -e "${YELLOW}Database not found. Creating...${NC}"
    mkdir -p ./data

    # Run backend to initialize DB
    if [ -f "./bin/goleapai-backend" ]; then
        ./bin/goleapai-backend migrate
        echo -e "${GREEN}✓ Database initialized${NC}"
    else
        echo -e "${YELLOW}Warning: Backend binary not found. Database may not be initialized.${NC}"
    fi
fi

# Parse arguments
CONFIG_FLAG=""
if [ ! -z "$1" ]; then
    CONFIG_FLAG="--config $1"
fi

# Run TUI
echo -e "${CYAN}Starting GoLeapAI TUI...${NC}"
echo ""

$TUI_BIN $CONFIG_FLAG
