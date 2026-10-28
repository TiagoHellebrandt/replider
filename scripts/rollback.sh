#!/bin/sh
# Rollback a Replider service to a previous image tag.
# Usage: ./rollback.sh <service> <image_tag>
# Example: ./rollback.sh api sha-abc1234
#
# WARNING: This does NOT rollback database migrations.
# If the deployment included schema changes, manual intervention may be needed.

set -e

SERVICE="$1"
TAG="$2"

if [ -z "$SERVICE" ] || [ -z "$TAG" ]; then
  echo "Usage: $0 <service> <image_tag>"
  echo "  service: api | web | app"
  echo "  image_tag: sha-abc1234 (from GHCR)"
  echo ""
  echo "Example: $0 api sha-abc1234"
  exit 1
fi

case "$SERVICE" in
  api)  IMAGE_VAR="API_IMAGE_TAG" ;;
  web)  IMAGE_VAR="WEB_IMAGE_TAG" ;;
  app)  IMAGE_VAR="APP_IMAGE_TAG" ;;
  *)    echo "Unknown service: $SERVICE (use: api, web, app)"; exit 1 ;;
esac

echo "[rollback] Rolling back $SERVICE to tag: $TAG"
echo "[rollback] Pulling image..."
export ${IMAGE_VAR}="$TAG"
docker compose -f ~/replider/docker-compose.prod.yml pull "$SERVICE"

echo "[rollback] Restarting $SERVICE..."
docker compose -f ~/replider/docker-compose.prod.yml up -d --no-deps "$SERVICE"

echo "[rollback] Verifying health..."
sleep 5
docker compose -f ~/replider/docker-compose.prod.yml ps "$SERVICE"

echo "[rollback] Done. Verify the service is working correctly."
echo "[rollback] WARNING: Database migrations were NOT rolled back."
echo "[rollback] If schema changes were involved, check db/migrations/ manually."
