#!/usr/bin/env bash
# deploy.sh — rsync to doylestonex, build natively on Pi (ARM64), restart via podman
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

# Rebuild image and restart container with host networking.
# podman-compose has a bug in v4.3.1 where it attaches the container to both
# --network host AND the auto-created project bridge, which Podman rejects.
# We use podman-compose for the build and podman run directly for the start.
echo "==> Rebuilding image"
ssh "$REMOTE_HOST" "cd $REMOTE_DIR && podman-compose build"

echo "==> Restarting container with host networking"
ssh "$REMOTE_HOST" "
  podman stop tools-onoffapi_onoffapi_1 2>/dev/null || true
  podman rm   tools-onoffapi_onoffapi_1 2>/dev/null || true
  podman run -d \
    --name tools-onoffapi_onoffapi_1 \
    --network host \
    --env-file $REMOTE_DIR/.env \
    -e PORT=8082 \
    -v /home/admin/.ssh/id_onoffapi_shutdown_doylestone02:/home/admin/.ssh/id_onoffapi_shutdown_doylestone02:ro \
    --restart unless-stopped \
    --health-cmd 'curl -sf http://localhost:8082/health' \
    --health-interval 30s \
    --health-timeout 10s \
    --health-retries 3 \
    --health-start-period 5s \
    localhost/tools-onoffapi_onoffapi:latest
"

# Health check
echo "==> Waiting for health check..."
sleep 5
ssh "$REMOTE_HOST" "curl -sf http://localhost:8082/health" && echo "  -> healthy" || echo "  -> health check FAILED"

echo "==> Deploy complete. API available at https://howapped.zapto.org/onoffapi/health"
