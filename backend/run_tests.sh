#!/bin/bash

# FastCRM Regression Test Runner
# Run this script from the backend directory

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================"
echo "   FastCRM Regression Test Suite"
echo "========================================"
echo ""

# Change to script directory
cd "$(dirname "$0")"

# Parse arguments
VERBOSE=""
COVER=""
RACE=""
SHORT=""
SPECIFIC=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose)
            VERBOSE="-v"
            shift
            ;;
        -c|--cover)
            COVER="-cover -coverprofile=coverage.out"
            shift
            ;;
        -r|--race)
            RACE="-race"
            shift
            ;;
        -s|--short)
            SHORT="-short"
            shift
            ;;
        -t|--test)
            SPECIFIC="-run $2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  -v, --verbose    Verbose output"
            echo "  -c, --cover      Generate coverage report"
            echo "  -r, --race       Enable race detection"
            echo "  -s, --short      Skip long-running tests"
            echo "  -t, --test NAME  Run specific test by name"
            echo "  -h, --help       Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                    Run all tests"
            echo "  $0 -v                 Run with verbose output"
            echo "  $0 -c                 Run with coverage"
            echo "  $0 -t TestAuth        Run only auth tests"
            echo "  $0 -v -c -r           Run verbose with coverage and race detection"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Build flags
FLAGS="$VERBOSE $COVER $RACE $SHORT $SPECIFIC"

echo -e "${YELLOW}Running tests...${NC}"
echo ""

# Run the tests
if go test ./tests/... $FLAGS -timeout 5m; then
    echo ""
    echo -e "${GREEN}========================================"
    echo "   All tests passed!"
    echo "========================================${NC}"

    # Generate coverage HTML if coverage was enabled
    if [[ -n "$COVER" ]]; then
        echo ""
        echo -e "${YELLOW}Generating coverage report...${NC}"
        go tool cover -html=coverage.out -o coverage.html
        echo -e "${GREEN}Coverage report saved to: coverage.html${NC}"

        # Show coverage summary
        echo ""
        echo "Coverage Summary:"
        go tool cover -func=coverage.out | tail -1
    fi
else
    echo ""
    echo -e "${RED}========================================"
    echo "   Some tests failed!"
    echo "========================================${NC}"
    exit 1
fi
