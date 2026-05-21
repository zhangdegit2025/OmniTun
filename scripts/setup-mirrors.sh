#!/bin/bash
# OmniTun - 容器镜像仓库代理配置脚本
# 用途: 在内网/离线环境下配置 Docker daemon 使用镜像代理
set -euo pipefail

MIRROR_DOCKER="v8z7mqygym93q0vv03.xuanyuan.run"
MIRROR_GHCR="v8z7mqygym93q0vv03-ghcr.xuanyuan.run"
MIRROR_QUAY="v8z7mqygym93q0vv03-quay.xuanyuan.run"

echo "========================================="
echo "  OmniTun - Registry Mirror Setup"
echo "========================================="
echo ""

# 检测 Docker 是否可用
DOCKER_AVAILABLE=false
if docker info &>/dev/null; then
    DOCKER_AVAILABLE=true
    echo "[OK] Docker daemon is running"
else
    echo "[WARN] Docker daemon not available - skipping daemon config"
fi

# 1. 配置 Docker daemon 镜像代理
if $DOCKER_AVAILABLE; then
    DOCKER_CONFIG="/etc/docker/daemon.json"
    
    if [ -f "$DOCKER_CONFIG" ]; then
        echo "[SKIP] $DOCKER_CONFIG already exists (merge manually if needed)"
    else
        echo "[ACTION] Creating $DOCKER_CONFIG with registry mirrors..."
        sudo tee "$DOCKER_CONFIG" > /dev/null <<'DAEMONJSON'
{
    "registry-mirrors": [
        "https://v8z7mqygym93q0vv03.xuanyuan.run"
    ],
    "insecure-registries": [],
    "log-driver": "json-file",
    "log-opts": {
        "max-size": "10m",
        "max-file": "3"
    }
}
DAEMONJSON
        echo "[OK] Docker daemon config created"
        echo "[ACTION] Restarting Docker daemon..."
        sudo systemctl restart docker 2>/dev/null || sudo service docker restart 2>/dev/null || echo "[WARN] Please restart Docker manually"
    fi
fi

# 2. 配置 .env 文件使用镜像代理
ENV_FILE=".env"
if [ ! -f "$ENV_FILE" ]; then
    cp .env.example "$ENV_FILE"
    echo "[OK] Created $ENV_FILE from .env.example"
fi

# 添加镜像代理配置到 .env
if ! grep -q "DOCKER_MIRROR" "$ENV_FILE" 2>/dev/null; then
    cat >> "$ENV_FILE" <<'ENVVARS'

# ---- 镜像仓库代理 ----
DOCKER_MIRROR=v8z7mqygym93q0vv03.xuanyuan.run/
GHCR_MIRROR=v8z7mqygym93q0vv03-ghcr.xuanyuan.run/
QUAY_MIRROR=v8z7mqygym93q0vv03-quay.xuanyuan.run/
ENVVARS
    echo "[OK] Added registry mirror config to $ENV_FILE"
else
    echo "[INFO] Registry mirror config already in $ENV_FILE"
fi

# 3. 测试镜像拉取
if $DOCKER_AVAILABLE; then
    echo ""
    echo "[TEST] Pulling postgres:16-alpine via mirror..."
    if docker pull ${MIRROR_DOCKER}/library/postgres:16-alpine 2>/dev/null; then
        echo "[OK] Docker Hub mirror working"
    else
        echo "[WARN] Docker Hub mirror pull failed - check network and mirror URL"
        echo "  Try: docker pull postgres:16-alpine (direct)"
    fi
fi

echo ""
echo "========================================="
echo "  Mirror Setup Complete"
echo "========================================="
echo ""
echo "Quick start:"
echo "  docker compose -f deploy/docker/docker-compose.yml up -d"
echo ""
echo "Available mirrors:"
echo "  Docker Hub:     $MIRROR_DOCKER"
echo "  GitHub CR:      $MIRROR_GHCR"
echo "  Quay.io:        $MIRROR_QUAY"
echo ""
echo "To disable mirrors, remove DOCKER_MIRROR/GHCR_MIRROR/QUAY_MIRROR from .env"
