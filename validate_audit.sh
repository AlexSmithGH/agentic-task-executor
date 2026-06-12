#!/bin/bash
set -e

echo "Validating AI/Agentic SDLC Readiness Audit..."
echo "=============================================="
echo ""

FAILED=0

# Check 1: .golangci.yml
echo -n "1. Checking .golangci.yml... "
if [ -f ".golangci.yml" ]; then
    echo "✅ PASS"
else
    echo "❌ FAIL"
    FAILED=1
fi

# Check 2: .pre-commit-config.yaml
echo -n "2. Checking .pre-commit-config.yaml... "
if [ -f ".pre-commit-config.yaml" ]; then
    # Verify it has lint, format, and secret detection
    if grep -q "golangci-lint" .pre-commit-config.yaml && \
       grep -q "gitleaks" .pre-commit-config.yaml && \
       grep -q "go-fmt" .pre-commit-config.yaml; then
        echo "✅ PASS"
    else
        echo "❌ FAIL (missing required hooks)"
        FAILED=1
    fi
else
    echo "❌ FAIL"
    FAILED=1
fi

# Check 3: Makefile targets
echo -n "3. Checking Makefile targets... "
if [ -f "Makefile" ]; then
    if grep -q "^test:" Makefile && \
       grep -q "^lint:" Makefile && \
       grep -q "^build:" Makefile; then
        echo "✅ PASS"
    else
        echo "❌ FAIL (missing required targets)"
        FAILED=1
    fi
else
    echo "❌ FAIL"
    FAILED=1
fi

# Check 4: CLAUDE.md or agents.md
echo -n "4. Checking CLAUDE.md or agents.md... "
if [ -f "CLAUDE.md" ] || [ -f "agents.md" ]; then
    echo "✅ PASS"
else
    echo "❌ FAIL"
    FAILED=1
fi

# Check 5: .claude/settings.json
echo -n "5. Checking .claude/settings.json... "
if [ -f ".claude/settings.json" ]; then
    # Verify it has hooks configuration
    if grep -q "hooks" .claude/settings.json; then
        echo "✅ PASS"
    else
        echo "❌ FAIL (missing hooks configuration)"
        FAILED=1
    fi
else
    echo "❌ FAIL"
    FAILED=1
fi

# Check 6: CI configuration
echo -n "6. Checking CI configuration... "
if [ -d ".github/workflows" ] && [ -f ".github/workflows/ci.yml" ]; then
    echo "✅ PASS"
else
    echo "❌ FAIL"
    FAILED=1
fi

# Check 7: README.md
echo -n "7. Checking README.md... "
if [ -f "README.md" ]; then
    # Verify it has project description and setup instructions
    if grep -qi "overview\|description" README.md && \
       grep -qi "setup\|install\|quick start" README.md; then
        echo "✅ PASS"
    else
        echo "❌ FAIL (missing description or setup instructions)"
        FAILED=1
    fi
else
    echo "❌ FAIL"
    FAILED=1
fi

# Check 8: Dockerfile with multi-stage build
echo -n "8. Checking Dockerfile... "
if [ -f "Dockerfile" ]; then
    # Check for multi-stage build (should have "AS builder" or similar)
    if grep -qi "AS.*builder\|AS.*build" Dockerfile; then
        echo "✅ PASS"
    else
        echo "❌ FAIL (not a multi-stage build)"
        FAILED=1
    fi
else
    echo "❌ FAIL"
    FAILED=1
fi

# Check 9: go.mod
echo -n "9. Checking go.mod... "
if [ -f "go.mod" ]; then
    # Verify it has a proper module path
    if grep -q "^module " go.mod; then
        echo "✅ PASS"
    else
        echo "❌ FAIL (missing module declaration)"
        FAILED=1
    fi
else
    echo "❌ FAIL"
    FAILED=1
fi

# Check 10: .env.example
echo -n "10. Checking .env.example... "
if [ -f ".env.example" ]; then
    echo "✅ PASS"
else
    echo "❌ FAIL"
    FAILED=1
fi

echo ""
echo "=============================================="
if [ $FAILED -eq 0 ]; then
    echo "✅ ALL CHECKS PASSED"
    exit 0
else
    echo "❌ SOME CHECKS FAILED"
    exit 1
fi
