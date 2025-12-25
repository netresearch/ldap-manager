#!/bin/bash
# Mutation Testing Script
# Runs go-mutesting on core packages to verify test quality

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Running Mutation Testing${NC}"
echo "========================================"

# Check if go-mutesting is installed
if ! command -v go-mutesting &> /dev/null; then
    echo -e "${YELLOW}Installing go-mutesting...${NC}"
    go install github.com/zimmski/go-mutesting/cmd/go-mutesting@v1.2
fi

# Packages to test
PACKAGES=(
    "./internal/ldap_cache"
    "./internal/options"
)

# Track overall results
TOTAL_MUTANTS=0
TOTAL_KILLED=0
TOTAL_SURVIVED=0

# Create output directory
mkdir -p ./mutation-reports

for pkg in "${PACKAGES[@]}"; do
    echo -e "\n${YELLOW}Testing package: ${pkg}${NC}"
    echo "----------------------------------------"

    # Run mutation testing
    OUTPUT_FILE="./mutation-reports/$(echo "$pkg" | tr '/' '_').txt"

    go-mutesting \
        --do-not-remove-tmp-folder=false \
        --verbose \
        "$pkg" 2>&1 | tee "$OUTPUT_FILE" || true

    # Parse results
    if [ -f "$OUTPUT_FILE" ]; then
        KILLED=$(grep -c "PASS" "$OUTPUT_FILE" || echo "0")
        SURVIVED=$(grep -c "FAIL" "$OUTPUT_FILE" || echo "0")
        MUTANTS=$((KILLED + SURVIVED))

        TOTAL_MUTANTS=$((TOTAL_MUTANTS + MUTANTS))
        TOTAL_KILLED=$((TOTAL_KILLED + KILLED))
        TOTAL_SURVIVED=$((TOTAL_SURVIVED + SURVIVED))

        echo -e "Package Results: ${KILLED} killed, ${SURVIVED} survived"
    fi
done

echo ""
echo "========================================"
echo -e "${YELLOW}Mutation Testing Summary${NC}"
echo "========================================"
echo "Total Mutants:  $TOTAL_MUTANTS"
echo "Killed:         $TOTAL_KILLED"
echo "Survived:       $TOTAL_SURVIVED"

if [ $TOTAL_MUTANTS -gt 0 ]; then
    KILL_RATE=$(echo "scale=2; $TOTAL_KILLED * 100 / $TOTAL_MUTANTS" | bc)
    echo "Kill Rate:      ${KILL_RATE}%"

    # Check threshold
    THRESHOLD=80
    if (( $(echo "$KILL_RATE >= $THRESHOLD" | bc -l) )); then
        echo -e "\n${GREEN}✅ Mutation score meets threshold (${THRESHOLD}%)${NC}"
        exit 0
    else
        echo -e "\n${RED}❌ Mutation score below threshold (${THRESHOLD}%)${NC}"
        echo "Consider adding more tests to kill surviving mutants."
        exit 1
    fi
else
    echo -e "${YELLOW}No mutants generated${NC}"
    exit 0
fi
