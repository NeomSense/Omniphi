#!/bin/bash

# Omniphi Test Runner for Ubuntu
# Usage: ./run_tests.sh [package]
# Examples:
#   ./run_tests.sh              # Run all tests
#   ./run_tests.sh address      # Run address tests only
#   ./run_tests.sh tokenomics   # Run tokenomics tests only

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Banner
echo -e "${BLUE}"
echo "╔════════════════════════════════════════╗"
echo "║     Omniphi Blockchain Test Suite     ║"
echo "╚════════════════════════════════════════╝"
echo -e "${NC}"

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed${NC}"
    echo "Please install Go 1.21+ first:"
    echo "  wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz"
    echo "  sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz"
    exit 1
fi

# Show Go version
echo -e "${BLUE}Go version:${NC} $(go version)"
echo ""

# Determine what to test
PACKAGE=$1

run_test() {
    local name=$1
    local path=$2

    echo -e "${YELLOW}Testing: $name${NC}"
    echo "Package: $path"
    echo ""

    if go test $path -v -cover; then
        echo -e "${GREEN}✓ $name tests PASSED${NC}"
        echo ""
        return 0
    else
        echo -e "${RED}✗ $name tests FAILED${NC}"
        echo ""
        return 1
    fi
}

# Test results
TOTAL=0
PASSED=0
FAILED=0

case $PACKAGE in
    "address")
        echo -e "${BLUE}Running Address Utilities Tests${NC}"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo ""
        if run_test "Address Utilities" "./pkg/address"; then
            ((PASSED++))
        else
            ((FAILED++))
        fi
        ((TOTAL++))
        ;;

    "tokenomics")
        echo -e "${BLUE}Running Tokenomics Tests${NC}"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo ""
        if run_test "Tokenomics Module" "./x/tokenomics/keeper"; then
            ((PASSED++))
        else
            ((FAILED++))
        fi
        ((TOTAL++))
        ;;

    "poc")
        echo -e "${BLUE}Running PoC Tests${NC}"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo ""
        if run_test "PoC Module" "./x/poc/keeper"; then
            ((PASSED++))
        else
            ((FAILED++))
        fi
        ((TOTAL++))
        ;;

    "coverage")
        echo -e "${BLUE}Running Tests with Coverage${NC}"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo ""
        echo "Generating coverage report..."
        go test ./... -coverprofile=coverage.out
        go tool cover -func=coverage.out
        echo ""
        echo "Generating HTML coverage report..."
        go tool cover -html=coverage.out -o coverage.html
        echo -e "${GREEN}✓ Coverage report saved to coverage.html${NC}"
        echo ""
        echo "Open with: firefox coverage.html"
        exit 0
        ;;

    *)
        echo -e "${BLUE}Running All Tests${NC}"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo ""

        # Address tests
        if run_test "Address Utilities" "./pkg/address"; then
            ((PASSED++))
        else
            ((FAILED++))
        fi
        ((TOTAL++))

        # Tokenomics tests
        if run_test "Tokenomics Module" "./x/tokenomics/keeper"; then
            ((PASSED++))
        else
            ((FAILED++))
        fi
        ((TOTAL++))

        # PoC tests
        if run_test "PoC Module" "./x/poc/keeper"; then
            ((PASSED++))
        else
            ((FAILED++))
        fi
        ((TOTAL++))
        ;;
esac

# Summary
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${BLUE}Test Summary${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Total:  $TOTAL test suites"
echo -e "${GREEN}Passed: $PASSED${NC}"
if [ $FAILED -gt 0 ]; then
    echo -e "${RED}Failed: $FAILED${NC}"
else
    echo "Failed: $FAILED"
fi
echo ""

# Exit code
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}╔════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║          ALL TESTS PASSED! ✓           ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════╝${NC}"
    exit 0
else
    echo -e "${RED}╔════════════════════════════════════════╗${NC}"
    echo -e "${RED}║         SOME TESTS FAILED! ✗           ║${NC}"
    echo -e "${RED}╚════════════════════════════════════════╝${NC}"
    exit 1
fi
