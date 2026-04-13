#!/usr/bin/env bash
# deploy.sh — rsync to doylestonex, build natively on Pi (ARM64), restart via podman-compose
#
# Usage:
#   make deploy
#   # or directly:
#   bash deploy/deploy.sh
#
# Requires:
#   - SSH access to doylestonex configured in ~/.ssh/config
#   - API_KEY set in .env at $REMOTE_DIR on doylestonex
#   - podman + podman-compose installed on doylestonex

set -euo pipefail

REMOTE_USER="admin"
REMOTE_HOST="doylestonex"
REMOTE_DIR="/home/admin/www/tools-onoffapi"
TRAEFIK_CONFIG_DIR="/home/admin/traefik/config/dynamic"

# Test SSH
echo "==> Testing SSH connection"
ssh "$REMOTE_HOST" "echo 'SSH OK'"

# Prune old images to save disk space
echo "==> Pruning old images on doylestonex"
ssh "$REMOTE_HOST" "podman image prune -f" || true

# Sync project files
echo "==> Syncing project to $REMOTE_HOST:$REMOTE_DIR"
rsync -avz --exclude='.git' \
           --exclude='bin/' \
           --exclude='.env' \
           ./ "$REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/"

# Install Traefik dynamic config
echo "==> Installing Traefik dynamic config"
scp deploy/onoffapi-traefik.yml "$REMOTE_USER@$REMOTE_HOST:$TRAEFIK_CONFIG_DIR/onoffapi.yml"

# Build on Pi (native ARM64)
echo "==> Building container on doylestonex"
GIT_COMMIT=$(git rev-parse --short HEAD)
ssh "$REMOTE_HOST" "cd $REMOTE_DIR && podman build --build-arg GIT_COMMIT=$GIT_COMMIT -f Dockerfile -t onoffapi:latest ."

# Restart container
echo "==> Restarting container"
ssh "$REMOTE_HOST" "cd $REMOTE_DIR && podman-compose down || true && podman-compose up -d"

# Health check
echo "==> Waiting for health check..."
sleep 5
ssh "$REMOTE_HOST" "curl -sf http://localhost:8082/health" && echo "  -> healthy" || echo "  -> health check FAILED"

echo "==> Deploy complete. API available at https://howapped.zapto.org/onoffapi/health"
