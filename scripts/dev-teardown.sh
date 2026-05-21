#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "[INFO] 停止 OmniTun 开发环境..."

docker compose \
    -f "$PROJECT_ROOT/deploy/docker/docker-compose.yml" \
    -f "$PROJECT_ROOT/deploy/docker/docker-compose.override.yml" \
    down --volumes --remove-orphans

echo ""
echo "[INFO] OmniTun 开发环境已停止并清理。"
echo ""
echo "[INFO] 如需保留数据，请使用以下命令:"
echo "  docker compose -f deploy/docker/docker-compose.yml -f deploy/docker/docker-compose.override.yml down"
