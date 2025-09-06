#!/bin/bash
# LDAP Manager - Comprehensive Testing Script
# Runs all tests with coverage reporting and quality checks

set -euo pipefail

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[0;33m'
readonly BLUE='\033[0;34m'
readonly RESET='\033[0m'

# Configuration
readonly COVERAGE_THRESHOLD=80
readonly COVERAGE_DIR="coverage-reports"
readonly COVERAGE_FILE="coverage.out"
readonly HTML_COVERAGE_FILE="${COVERAGE_DIR}/coverage.html"

# Create coverage directory
mkdir -p "$COVERAGE_DIR"

# Helper functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${RESET}"
}

log_success() {
    echo -e "${GREEN}✅ $1${RESET}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${RESET}"
}

log_error() {
    echo -e "${RED}❌ $1${RESET}"
}

# Function to run tests with coverage
run_tests() {
    log_info "Running tests with coverage..."
    
    # Run tests with coverage
    if go test -v -race -coverprofile="$COVERAGE_FILE" -covermode=atomic ./...; then
        log_success "All tests passed"
    else
        log_error "Some tests failed"
        return 1
    fi
}

# Function to generate coverage reports
generate_coverage_reports() {
    log_info "Generating coverage reports..."
    
    if [[ ! -f "$COVERAGE_FILE" ]]; then
        log_error "Coverage file not found"
        return 1
    fi
    
    # Generate HTML coverage report
    go tool cover -html="$COVERAGE_FILE" -o "$HTML_COVERAGE_FILE"
    log_success "HTML coverage report generated: $HTML_COVERAGE_FILE"
    
    # Generate text coverage summary
    local coverage_summary
    coverage_summary=$(go tool cover -func="$COVERAGE_FILE" | grep "^total:" | awk '{print $3}')
    
    if [[ -n "$coverage_summary" ]]; then
        local coverage_percentage
        coverage_percentage=${coverage_summary%\%}
        
        echo "Coverage Summary:" > "${COVERAGE_DIR}/summary.txt"
        echo "Total Coverage: $coverage_summary" >> "${COVERAGE_DIR}/summary.txt"
        echo "Threshold: ${COVERAGE_THRESHOLD}%" >> "${COVERAGE_DIR}/summary.txt"
        
        log_info "Coverage: $coverage_summary"
        
        # Check coverage threshold
        if (( $(echo "$coverage_percentage >= $COVERAGE_THRESHOLD" | bc -l) )); then
            log_success "Coverage threshold met ($coverage_summary >= ${COVERAGE_THRESHOLD}%)"
        else
            log_warning "Coverage below threshold ($coverage_summary < ${COVERAGE_THRESHOLD}%)"
            return 1
        fi
    else
        log_error "Could not determine coverage percentage"
        return 1
    fi
}

# Function to run benchmarks
run_benchmarks() {
    log_info "Running benchmarks..."
    
    if go test -bench=. -benchmem -run=^$ ./... > "${COVERAGE_DIR}/benchmarks.txt"; then
        log_success "Benchmarks completed"
    else
        log_warning "Some benchmarks failed or none found"
    fi
}

# Function to run race detection tests
run_race_tests() {
    log_info "Running race detection tests..."
    
    if go test -race -short ./...; then
        log_success "No race conditions detected"
    else
        log_error "Race conditions detected"
        return 1
    fi
}

# Function to run integration tests (if they exist)
run_integration_tests() {
    log_info "Looking for integration tests..."
    
    # Check if integration tests exist
    if find . -name "*integration*test.go" -o -name "*_integration.go" | grep -q .; then
        log_info "Running integration tests..."
        if go test -tags=integration -v ./...; then
            log_success "Integration tests passed"
        else
            log_error "Integration tests failed"
            return 1
        fi
    else
        log_info "No integration tests found"
    fi
}

# Function to check test files exist
check_test_coverage() {
    log_info "Checking test coverage across packages..."
    
    local packages_without_tests=()
    
    # Find all Go packages
    while IFS= read -r -d '' package_dir; do
        local package_path
        package_path=$(realpath --relative-to="." "$package_dir")
        
        # Skip vendor, node_modules, and generated files
        if [[ "$package_path" =~ (vendor|node_modules|.*_templ\.go) ]]; then
            continue
        fi
        
        # Check if package has Go files (excluding tests)
        if find "$package_dir" -maxdepth 1 -name "*.go" ! -name "*_test.go" | grep -q .; then
            # Check if package has test files
            if ! find "$package_dir" -maxdepth 1 -name "*_test.go" | grep -q .; then
                packages_without_tests+=("$package_path")
            fi
        fi
    done < <(find . -type d -exec test -e '{}/go.mod' -o -e '{}/*.go' \; -print0 | head -20)
    
    if [[ ${#packages_without_tests[@]} -gt 0 ]]; then
        log_warning "Packages without tests:"
        printf '%s\n' "${packages_without_tests[@]}"
    else
        log_success "All packages have test files"
    fi
}

# Function to clean up old coverage files
cleanup() {
    log_info "Cleaning up old coverage files..."
    find . -name "*.out" -name "coverage*" -delete 2>/dev/null || true
}

# Main execution
main() {
    log_info "Starting comprehensive test suite..."
    
    # Cleanup old files
    cleanup
    
    # Run different types of tests
    local exit_code=0
    
    if ! run_tests; then
        exit_code=1
    fi
    
    if ! generate_coverage_reports; then
        exit_code=1
    fi
    
    if ! run_race_tests; then
        exit_code=1
    fi
    
    # Optional tests (don't fail on these)
    run_benchmarks || log_warning "Benchmarks had issues"
    run_integration_tests || log_warning "Integration tests had issues" 
    check_test_coverage || log_warning "Test coverage check had issues"
    
    if [[ $exit_code -eq 0 ]]; then
        log_success "All critical tests passed!"
    else
        log_error "Some critical tests failed"
    fi
    
    return $exit_code
}

# Handle script arguments
case "${1:-}" in
    "help"|"-h"|"--help")
        echo "LDAP Manager Test Script"
        echo ""
        echo "Usage: $0 [command]"
        echo ""
        echo "Commands:"
        echo "  (no args)     Run all tests"
        echo "  coverage      Run tests and generate coverage reports"
        echo "  benchmarks    Run benchmarks only"
        echo "  race          Run race detection tests"
        echo "  integration   Run integration tests"
        echo "  clean         Clean up coverage files"
        echo "  help          Show this help"
        ;;
    "coverage")
        run_tests && generate_coverage_reports
        ;;
    "benchmarks")
        run_benchmarks
        ;;
    "race")
        run_race_tests
        ;;
    "integration")
        run_integration_tests
        ;;
    "clean")
        cleanup
        ;;
    *)
        main "$@"
        ;;
esac