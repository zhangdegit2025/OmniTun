#!/bin/bash
set -euo pipefail

TAG="${1:?Usage: $0 <tag>}"
echo "Releasing OmniTun ${TAG}..."

git tag "v${TAG}"
git push origin "v${TAG}"

echo "✓ Tag pushed. GitHub Actions will build the release."
