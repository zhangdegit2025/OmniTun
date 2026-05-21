#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if [ ! -f "$PROJECT_ROOT/.env" ]; then
    echo "[INFO] .env 文件不存在，从 .env.example 复制..."
    cp "$PROJECT_ROOT/.env.example" "$PROJECT_ROOT/.env"
    echo "[INFO] 已创建 .env 文件，请根据需要修改配置"
fi

echo "[INFO] 启动 OmniTun 开发环境..."

docker compose \
    -f "$PROJECT_ROOT/deploy/docker/docker-compose.yml" \
    -f "$PROJECT_ROOT/deploy/docker/docker-compose.override.yml" \
    --env-file "$PROJECT_ROOT/.env" \
    up -d --build

echo ""
echo "[INFO] 等待所有服务健康检查通过..."
sleep 5

echo ""
echo "[INFO] 服务状态:"
docker compose \
    -f "$PROJECT_ROOT/deploy/docker/docker-compose.yml" \
    -f "$PROJECT_ROOT/deploy/docker/docker-compose.override.yml" \
    ps

echo ""
echo "=============================================="
echo "  OmniTun 开发环境已就绪!"
echo "=============================================="
echo ""
echo "  访问地址:"
echo "  Dashboard:  http://localhost:3000"
echo "  API:        http://localhost:8080"
echo "  MinIO:      http://localhost:9001"
echo "  NATS:       http://localhost:8222"
echo "  ClickHouse: http://localhost:8123"
echo ""
echo "  停止环境: scripts/dev-teardown.sh"
echo "=============================================="
