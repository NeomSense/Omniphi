#!/bin/bash
# Comprehensive Test Runner for Omniphi Blockchain
# Runs all P0/P1/P2 tests from the comprehensive test program

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     Omniphi Comprehensive Test Suite Runner               ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Parse arguments
VERBOSE=false
COVERAGE=false
SPECIFIC_TEST=""
PRIORITY="all"

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -c|--coverage)
            COVERAGE=true
            shift
            ;;
        -t|--test)
            SPECIFIC_TEST="$2"
            shift 2
            ;;
        -p|--priority)
            PRIORITY="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  -v, --verbose        Enable verbose output"
            echo "  -c, --coverage       Generate coverage report"
            echo "  -t, --test NAME      Run specific test (e.g., TestTC001_HardCapEnforcementAtBoundary)"
            echo "  -p, --priority LEVEL Run tests by priority (p0, p1, p2, all)"
            echo "  -h, --help           Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                          # Run all tests"
            echo "  $0 --priority p0            # Run only P0 tests"
            echo "  $0 --test TestTC001         # Run specific test"
            echo "  $0 --coverage --verbose     # Run with coverage and verbose output"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Change to project root
cd "$PROJECT_ROOT"

echo -e "${YELLOW}Project Root: ${NC}$PROJECT_ROOT"
echo -e "${YELLOW}Test Mode: ${NC}${PRIORITY^^}"
echo ""

# Function to run tests with appropriate flags
run_tests() {
    local test_path=$1
    local test_name=$2
    local flags=""

    if [ "$VERBOSE" = true ]; then
        flags="$flags -v"
    fi

    if [ "$COVERAGE" = true ]; then
        flags="$flags -coverprofile=coverage.out -covermode=atomic"
    fi

    if [ -n "$SPECIFIC_TEST" ]; then
        flags="$flags -run $SPECIFIC_TEST"
    fi

    echo -e "${BLUE}Running: ${NC}$test_name"

    if go test $flags -timeout 30m "$test_path"; then
        echo -e "${GREEN}✓ PASSED: ${NC}$test_name"
        return 0
    else
        echo -e "${RED}✗ FAILED: ${NC}$test_name"
        return 1
    fi
}

# Track results
TOTAL_SUITES=0
PASSED_SUITES=0
FAILED_SUITES=0

# Run P0 Tests (Critical)
if [ "$PRIORITY" = "all" ] || [ "$PRIORITY" = "p0" ]; then
    echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${YELLOW}  P0 Tests (Critical - Must Pass 100%)${NC}"
    echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
    echo ""

    # Supply and Monetary Policy Tests (TC-001 to TC-013)
    TOTAL_SUITES=$((TOTAL_SUITES + 1))
    if run_tests "./test/comprehensive" "Supply & Monetary Policy Tests"; then
        PASSED_SUITES=$((PASSED_SUITES + 1))
    else
        FAILED_SUITES=$((FAILED_SUITES + 1))
    fi
    echo ""

    # Governance Tests (TC-022 to TC-033)
    TOTAL_SUITES=$((TOTAL_SUITES + 1))
    if run_tests "./test/comprehensive" "Governance Tests"; then
        PASSED_SUITES=$((PASSED_SUITES + 1))
    else
        FAILED_SUITES=$((FAILED_SUITES + 1))
    fi
    echo ""
fi

# Run existing module tests
if [ "$PRIORITY" = "all" ]; then
    echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${YELLOW}  Existing Module Tests${NC}"
    echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
    echo ""

    # PoC Module Tests
    TOTAL_SUITES=$((TOTAL_SUITES + 1))
    if run_tests "./x/poc/..." "PoC Module Tests"; then
        PASSED_SUITES=$((PASSED_SUITES + 1))
    else
        FAILED_SUITES=$((FAILED_SUITES + 1))
    fi
    echo ""

    # Tokenomics Module Tests
    TOTAL_SUITES=$((TOTAL_SUITES + 1))
    if run_tests "./x/tokenomics/..." "Tokenomics Module Tests"; then
        PASSED_SUITES=$((PASSED_SUITES + 1))
    else
        FAILED_SUITES=$((FAILED_SUITES + 1))
    fi
    echo ""

    # Address Utilities Tests
    TOTAL_SUITES=$((TOTAL_SUITES + 1))
    if run_tests "./pkg/address/..." "Address Utilities Tests"; then
        PASSED_SUITES=$((PASSED_SUITES + 1))
    else
        FAILED_SUITES=$((FAILED_SUITES + 1))
    fi
    echo ""
fi

# Generate coverage report if requested
if [ "$COVERAGE" = true ] && [ -f "coverage.out" ]; then
    echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${YELLOW}  Coverage Report${NC}"
    echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
    echo ""

    go tool cover -func=coverage.out | tail -n 1

    # Generate HTML report
    go tool cover -html=coverage.out -o coverage.html
    echo -e "${GREEN}✓ HTML coverage report generated: ${NC}coverage.html"
    echo ""
fi

# Summary
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}  Test Summary${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "Total Suites:  $TOTAL_SUITES"
echo -e "${GREEN}Passed:        $PASSED_SUITES${NC}"

if [ $FAILED_SUITES -gt 0 ]; then
    echo -e "${RED}Failed:        $FAILED_SUITES${NC}"
else
    echo -e "Failed:        $FAILED_SUITES"
fi

echo ""

# Exit code
if [ $FAILED_SUITES -eq 0 ]; then
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║                  ALL TESTS PASSED! ✓                       ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
    exit 0
else
    echo -e "${RED}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${RED}║              SOME TESTS FAILED! ✗                          ║${NC}"
    echo -e "${RED}╚════════════════════════════════════════════════════════════╝${NC}"
    exit 1
fi
