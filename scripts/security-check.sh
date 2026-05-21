#!/bin/bash
# OmniTun Security Quick Check
echo "=== Checking for hardcoded secrets ==="
# Exclude test files and config templates
if grep -rn "sk_[a-z]*_[a-zA-Z0-9]\{20,\}" cmd/ internal/ pkg/ --include="*.go" 2>/dev/null | grep -v "config\.go\|_test\.go\|example"; then
    echo "WARNING: Potential API keys found in source code"
else
    echo "OK: No hardcoded API keys found"
fi

echo "=== Checking for hardcoded passwords ==="
if grep -rn "password\s*=\s*\"[^\"]\{4,\}\"" cmd/ internal/ pkg/ --include="*.go" 2>/dev/null | grep -v "_test\.go\|config\.go\|example"; then
    echo "WARNING: Potential hardcoded passwords found"
else
    echo "OK: No hardcoded passwords found"
fi

echo "=== Go vet security checks ==="
go vet ./...
echo "=== Done ==="
