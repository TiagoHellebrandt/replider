#!/usr/bin/env bash
# =============================================================================
# init-ssl.sh — First-time Let's Encrypt SSL bootstrap
#
# Run this ONCE on the VPS after cloning the repo and setting up DNS.
# Prerequisites:
#   - DNS A records pointing to this VPS:
#       replider.com, www.replider.com, api.replider.com, app.replider.com
#   - Docker + Docker Compose installed
#   - ~/replider/docker-compose.prod.yml present (deployed by GitHub Actions
#     or copied manually)
#   - ~/replider/.env present with at least GITHUB_REPOSITORY_OWNER set
#
# Usage:
#   cd ~/replider
#   bash scripts/init-ssl.sh
# =============================================================================
set -euo pipefail

DOMAINS="replider.com www.replider.com api.replider.com app.replider.com"
CERTBOT_DOMAINS="-d replider.com -d www.replider.com -d api.replider.com -d app.replider.com"
EMAIL="you@example.com"          # <-- change to your email
CERT_PATH="/etc/letsencrypt/live/replider.com"
COMPOSE="docker compose -f docker-compose.prod.yml"

# ── Helpers ───────────────────────────────────────────────────────────────────
info()    { echo "[init-ssl] $*"; }
success() { echo "[init-ssl] ✓ $*"; }
die()     { echo "[init-ssl] ERROR: $*" >&2; exit 1; }

# ── Pre-flight ────────────────────────────────────────────────────────────────
command -v docker >/dev/null 2>&1 || die "Docker is not installed."
[ -f docker-compose.prod.yml ]    || die "docker-compose.prod.yml not found. Run from ~/replider."

info "Creating Docker volumes (idempotent)..."
docker volume create replider_certbot_www   2>/dev/null || true
docker volume create replider_certbot_certs 2>/dev/null || true

# ── Check if real certs already exist ─────────────────────────────────────────
if docker run --rm \
     -v replider_certbot_certs:/etc/letsencrypt:ro \
     alpine:3.20 \
     test -f "${CERT_PATH}/fullchain.pem" 2>/dev/null; then
  success "Certificates already exist. Skipping bootstrap."
  info "To force renewal: docker compose exec certbot certbot renew --force-renewal"
  exit 0
fi

# ── Step 1: Create temporary self-signed cert so nginx can start ──────────────
info "Creating temporary self-signed certificate..."
docker run --rm \
  -v replider_certbot_certs:/etc/letsencrypt \
  alpine:3.20 sh -c "
    apk add --no-cache openssl >/dev/null 2>&1
    mkdir -p ${CERT_PATH}
    openssl req -x509 -nodes -newkey rsa:2048 -days 1 \
      -keyout ${CERT_PATH}/privkey.pem \
      -out    ${CERT_PATH}/fullchain.pem \
      -subj '/CN=localhost' 2>/dev/null
    cp ${CERT_PATH}/fullchain.pem ${CERT_PATH}/chain.pem
  "
success "Dummy certificate created."

# ── Step 2: Start nginx only (serves HTTP for the ACME challenge) ─────────────
info "Starting nginx..."
$COMPOSE up -d nginx
sleep 4   # allow nginx to fully initialize

# Verify nginx is reachable
if ! docker exec replider_nginx nginx -t >/dev/null 2>&1; then
  die "nginx failed to start — check 'docker logs replider_nginx'"
fi
success "nginx is up."

# ── Step 3: Obtain real Let's Encrypt certificate via HTTP-01 challenge ────────
info "Requesting Let's Encrypt certificate for: ${DOMAINS}..."
docker run --rm \
  -v replider_certbot_www:/var/www/certbot \
  -v replider_certbot_certs:/etc/letsencrypt \
  certbot/certbot certonly \
    --webroot \
    --webroot-path=/var/www/certbot \
    --email "${EMAIL}" \
    --agree-tos \
    --no-eff-email \
    --non-interactive \
    ${CERTBOT_DOMAINS}

success "Real certificates obtained."

# ── Step 4: Reload nginx with real certificates ────────────────────────────────
info "Reloading nginx configuration..."
docker exec replider_nginx nginx -s reload
success "nginx reloaded with Let's Encrypt certificates."

# ── Step 5: Start remaining services ──────────────────────────────────────────
info "Starting all services..."
$COMPOSE up -d
success "All services started."

echo ""
echo "================================================================"
echo " SSL bootstrap complete!"
echo " replider.com       → https://replider.com"
echo " api.replider.com   → https://api.replider.com"
echo " app.replider.com   → https://app.replider.com"
echo ""
echo " Automatic renewal is handled by the certbot container."
echo " Check status: docker compose logs certbot"
echo "================================================================"
