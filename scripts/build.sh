#!/bin/bash
set -euo pipefail

TAG="${1:-dev}"
BUILD_DIR="dist/${TAG}"

echo "Building OmniTun ${TAG}..."

for cmd in server relay client admin; do
    CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=${TAG}" \
        -o "${BUILD_DIR}/omnitun-${cmd}" "./cmd/${cmd}/"
done

cd web && npm run build
cp -r dist "../${BUILD_DIR}/web"

echo "✓ Build complete: ${BUILD_DIR}/"
