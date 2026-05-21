#!/bin/bash
# OmniTun Production Deploy Script
# Usage: bash scripts/deploy.sh [start|stop|status|logs]
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEPLOY_DIR="${PROJECT_ROOT}/deploy"
CONFIG_DIR="/etc/omnitun"
DATA_DIR="/var/lib/omnitun"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[OMNITUN]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

CMD="${1:-start}"

# ==========================================
# 1. Pre-flight checks
# ==========================================
preflight() {
    log "Running pre-flight checks..."

    command -v docker >/dev/null 2>&1 || err "Docker is required but not installed"
    docker info >/dev/null 2>&1 || err "Docker daemon is not running"

    if [ ! -f "${DEPLOY_DIR}/.env.prod" ]; then
        warn "No .env.prod found, copying from .env.example"
        cp "${PROJECT_ROOT}/.env.example" "${DEPLOY_DIR}/.env.prod"
    fi

    # Check for JWT keys
    if [ ! -f "${DEPLOY_DIR}/keys/jwt_rsa_private.pem" ]; then
        warn "JWT keys not found, generating..."
        mkdir -p "${DEPLOY_DIR}/keys"
        openssl genrsa -out "${DEPLOY_DIR}/keys/jwt_rsa_private.pem" 2048 2>/dev/null || true
        openssl rsa -in "${DEPLOY_DIR}/keys/jwt_rsa_private.pem" -pubout -out "${DEPLOY_DIR}/keys/jwt_rsa_public.pem" 2>/dev/null || true
    fi

    log "Pre-flight checks passed"
}

# ==========================================
# 2. Setup directories
# ==========================================
setup_dirs() {
    log "Setting up directories..."
    sudo mkdir -p "${CONFIG_DIR}/keys"
    sudo mkdir -p "${CONFIG_DIR}/certs"
    sudo mkdir -p "${DATA_DIR}"
    sudo cp "${DEPLOY_DIR}/config.prod.yaml" "${CONFIG_DIR}/config.yaml" 2>/dev/null || true
    sudo cp "${DEPLOY_DIR}/keys/"* "${CONFIG_DIR}/keys/" 2>/dev/null || true
    sudo chmod 600 "${CONFIG_DIR}/keys/"*
    sudo chown -R 65532:65532 "${DATA_DIR}" 2>/dev/null || true
    log "Directories ready"
}

# ==========================================
# 3. Start infrastructure
# ==========================================
start_infra() {
    log "Starting infrastructure services..."
    docker compose -f "${DEPLOY_DIR}/docker/docker-compose.yml" --profile infra up -d \
        postgres valkey nats clickhouse minio

    log "Waiting for services to be healthy..."
    for svc in postgres valkey nats clickhouse minio; do
        for i in $(seq 1 30); do
            if docker compose -f "${DEPLOY_DIR}/docker/docker-compose.yml" ps "$svc" | grep -q "healthy"; then
                log "  ✓ $svc is healthy"
                break
            fi
            sleep 2
        done
    done
}

# ==========================================
# 4. Run migrations
# ==========================================
run_migrations() {
    log "Running database migrations..."
    docker run --rm --network omnitun-backend \
        -e "DATABASE_URL=${DATABASE_URL:-postgres://omnitun:changeme@postgres:5432/omnitun?sslmode=disable}" \
        -v "${PROJECT_ROOT}/migrations:/migrations" \
        migrate/migrate \
        -path /migrations/pg -database "$DATABASE_URL" up 2>/dev/null || warn "PG migration skipped (migrate CLI not available)"
}

# ==========================================
# 5. Start OmniTun services
# ==========================================
start_omnitun() {
    log "Starting OmniTun services..."
    docker compose -f "${DEPLOY_DIR}/docker/docker-compose.yml" --profile app up -d \
        api-gateway orchestrator relay web

    log "Waiting for OmniTun to be ready..."
    for i in $(seq 1 20); do
        if curl -sf http://localhost:8080/health >/dev/null 2>&1; then
            log "  ✓ API Server is ready"
            break
        fi
        sleep 3
    done
}

# ==========================================
# 6. Status check
# ==========================================
status() {
    log "OmniTun Status:"
    docker compose -f "${DEPLOY_DIR}/docker/docker-compose.yml" ps

    echo ""
    echo "Endpoints:"
    echo "  API:       http://localhost:8080"
    echo "  WebSocket: wss://localhost:9443"
    echo "  Dashboard: http://localhost:3000"
    echo "  Metrics:   http://localhost:9090/metrics"
    echo "  MinIO:     http://localhost:9001"
    echo "  NATS:      http://localhost:8222"
}

# ==========================================
# Main
# ==========================================
case "$CMD" in
    start)
        preflight
        setup_dirs
        start_infra
        run_migrations
        start_omnitun
        status
        log "OmniTun deployment complete!"
        ;;
    stop)
        log "Stopping OmniTun..."
        docker compose -f "${DEPLOY_DIR}/docker/docker-compose.yml" down
        log "OmniTun stopped"
        ;;
    status)
        status
        ;;
    logs)
        docker compose -f "${DEPLOY_DIR}/docker/docker-compose.yml" logs -f --tail=100
        ;;
    *)
        echo "Usage: $0 {start|stop|status|logs}"
        exit 1
        ;;
esac
