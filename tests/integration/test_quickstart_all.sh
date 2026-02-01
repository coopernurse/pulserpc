#!/bin/bash
# Orchestrator script that runs all quickstart tests
# This script tests each language quickstart guide end-to-end

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
LANGUAGES=("go" "python" "java" "typescript" "csharp")
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FAILED_TESTS=()

echo -e "${BLUE}╔═══════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   PulseRPC Quickstart Guide Test Suite              ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════╝${NC}"
echo ""

# Track results
RESULTS=()

for lang in "${LANGUAGES[@]}"; do
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Testing $lang quickstart...${NC}"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    TEST_SCRIPT="$SCRIPT_DIR/test_quickstart_${lang}.sh"

    if [ ! -f "$TEST_SCRIPT" ]; then
        echo -e "${RED}✗ Test script not found: $TEST_SCRIPT${NC}"
        FAILED_TESTS+=("$lang (script not found)")
        RESULTS+=("[$lang] ${RED}FAILED${NC} - script not found")
        continue
    fi

    if bash "$TEST_SCRIPT"; then
        echo ""
        echo -e "${GREEN}✓ $lang quickstart test PASSED${NC}"
        RESULTS+=("[$lang] ${GREEN}PASSED${NC}")
    else
        EXIT_CODE=$?
        echo ""
        echo -e "${RED}✗ $lang quickstart test FAILED (exit code: $EXIT_CODE)${NC}"
        FAILED_TESTS+=("$lang")
        RESULTS+=("[$lang] ${RED}FAILED${NC} (exit code: $EXIT_CODE)")
    fi
    echo ""
done

# Summary
echo -e "${BLUE}╔═══════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   Test Summary                                        ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════╝${NC}"
echo ""

for result in "${RESULTS[@]}"; do
    echo -e "  $result"
done

echo ""

if [ ${#FAILED_TESTS[@]} -eq 0 ]; then
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║   ✓ All quickstart tests PASSED!                     ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════╝${NC}"
    exit 0
else
    echo -e "${RED}╔═══════════════════════════════════════════════════════╗${NC}"
    echo -e "${RED}║   ✗ Some quickstart tests FAILED                     ║${NC}"
    echo -e "${RED}╚═══════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${RED}Failed tests: ${FAILED_TESTS[*]}${NC}"
    exit 1
fi
