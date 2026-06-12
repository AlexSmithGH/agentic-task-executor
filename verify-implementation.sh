#!/bin/bash
# Verification script for AI/Agentic SDLC readiness improvements
# This script checks that all required files are present and valid

set -e

echo "==================================================================="
echo "AI/Agentic SDLC Readiness Verification"
echo "==================================================================="
echo ""

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counters
PASSED=0
FAILED=0

# Function to check if file exists
check_file() {
    if [ -f "$1" ]; then
        echo -e "${GREEN}✓${NC} $1 exists"
        ((PASSED++))
        return 0
    else
        echo -e "${RED}✗${NC} $1 missing"
        ((FAILED++))
        return 1
    fi
}

# Function to validate JSON
validate_json() {
    if python3 -m json.tool "$1" > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} $1 is valid JSON"
        ((PASSED++))
        return 0
    else
        echo -e "${RED}✗${NC} $1 has invalid JSON"
        ((FAILED++))
        return 1
    fi
}

# Function to check file size
check_file_size() {
    local size=$(wc -c < "$1" | tr -d ' ')
    if [ "$size" -gt "$2" ]; then
        echo -e "${GREEN}✓${NC} $1 has adequate content (${size} bytes)"
        ((PASSED++))
        return 0
    else
        echo -e "${RED}✗${NC} $1 is too small (${size} bytes, expected > $2)"
        ((FAILED++))
        return 1
    fi
}

echo "1. Checking Core Configuration Files"
echo "-------------------------------------------------------------------"
check_file ".golangci.yml"
check_file_size ".golangci.yml" 2000

check_file ".pre-commit-config.yaml"
check_file_size ".pre-commit-config.yaml" 2000

check_file "CLAUDE.md"
check_file_size "CLAUDE.md" 8000

check_file ".markdownlint.json"
validate_json ".markdownlint.json"

echo ""
echo "2. Checking Claude Integration"
echo "-------------------------------------------------------------------"
check_file ".claude/settings.json"
validate_json ".claude/settings.json"
check_file_size ".claude/settings.json" 3000

echo ""
echo "3. Checking GitHub Configuration"
echo "-------------------------------------------------------------------"
check_file ".github/workflows/ci.yml"
check_file_size ".github/workflows/ci.yml" 3000

check_file ".github/workflows/release.yml"
check_file_size ".github/workflows/release.yml" 4000

check_file ".github/dependabot.yml"
check_file_size ".github/dependabot.yml" 800

check_file ".github/CODEOWNERS"

check_file ".github/PULL_REQUEST_TEMPLATE.md"
check_file_size ".github/PULL_REQUEST_TEMPLATE.md" 2000

check_file ".github/ISSUE_TEMPLATE/bug_report.yml"
check_file_size ".github/ISSUE_TEMPLATE/bug_report.yml" 2000

check_file ".github/ISSUE_TEMPLATE/feature_request.yml"
check_file_size ".github/ISSUE_TEMPLATE/feature_request.yml" 2000

echo ""
echo "4. Checking Existing Files (Should Be Unchanged)"
echo "-------------------------------------------------------------------"
check_file "Makefile"
check_file "README.md"
check_file "Dockerfile"
check_file "go.mod"
check_file ".env.example"
check_file "docker-compose.yml"

echo ""
echo "5. Checking Documentation Structure"
echo "-------------------------------------------------------------------"
check_file "docs/ARCHITECTURE.md"
check_file "docs/GETTING_STARTED.md"
check_file "docs/PROJECT_STATUS.md"
check_file "docs/QUICK_REFERENCE.md"

echo ""
echo "6. Testing Makefile Targets"
echo "-------------------------------------------------------------------"
if make help > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} make help works"
    ((PASSED++))
else
    echo -e "${RED}✗${NC} make help failed"
    ((FAILED++))
fi

echo ""
echo "7. Checking Git Integration"
echo "-------------------------------------------------------------------"
if [ -d ".git" ]; then
    echo -e "${GREEN}✓${NC} Git repository initialized"
    ((PASSED++))
else
    echo -e "${RED}✗${NC} Not a git repository"
    ((FAILED++))
fi

echo ""
echo "==================================================================="
echo "Verification Results"
echo "==================================================================="
echo -e "Passed: ${GREEN}${PASSED}${NC}"
echo -e "Failed: ${RED}${FAILED}${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All checks passed! Repository is ready for AI/agentic SDLC.${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Install pre-commit hooks: pip install pre-commit && pre-commit install"
    echo "2. Run initial check: pre-commit run --all-files"
    echo "3. Review CLAUDE.md for AI agent guidance"
    echo "4. Review .claude/settings.json for IDE integration"
    echo "5. Push changes to trigger CI/CD pipeline"
    exit 0
else
    echo -e "${RED}✗ Some checks failed. Please review the errors above.${NC}"
    exit 1
fi
